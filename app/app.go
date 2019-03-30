package app

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/aphistic/gomol"
	gomolconsole "github.com/aphistic/gomol-console"
)

const (
	timestamp   = `{{.Timestamp.Format "2006-01-02 15:04:05.000"}} `
	logTemplate = `[{{color}}{{ucase .LevelName}}{{reset}}] {{.Message}}{{if .Attrs}} {{json .Attrs}}{{end}}`
)

// App represents a core application instance. Values can be mocked for testing.
type App struct {
	Arguments   []string        // Command Line arguments
	Environment []string        // OS Environment Variables
	Context     context.Context // Application context
	Stdin       io.Reader       // fd0 /dev/stdin
	Stdout      io.Writer       // fd1 /dev/stdout
	Stderr      io.Writer       // fd2 /dev/stderr
	ExitHandler func(int)       // handler for calls to os.Exit

	loggerMu sync.Mutex
	logger   *gomol.Base

	errchMu sync.Mutex
	errch   chan error
}

// New returns a new App instance. The values are take directly from the environment. Manually construct
// an App instance in order to mock these values.
func New() *App {
	return &App{
		Arguments:   os.Args[1:],
		Environment: os.Environ(),
		Context:     context.Background(),
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		ExitHandler: os.Exit,
	}
}

// Exit calls the app ExitHandler. If no ExitHandler is set, calls os.Exit. This method properly shuts down the app
// logger if it has been initialized.
func (a *App) Exit(code int) {
	a.loggerMu.Lock()
	if a.logger != nil {
		if a.logger.IsInitialized() {
			if err := a.logger.ShutdownLoggers(); err != nil {
				panic(err)
			}
		}
		a.logger = nil
	}
	a.loggerMu.Unlock()

	if a.ExitHandler == nil {
		os.Exit(code)
	} else {
		a.ExitHandler(code)
		panic("exit handler returned")
	}
}

// Logger returns a cached logger instance. ShutdownLoggers must be called on the logger before terminating the app.
func (a *App) Logger() *gomol.Base {
	a.loggerMu.Lock()
	defer a.loggerMu.Unlock()

	if a.logger == nil {
		consoleConfig := gomolconsole.ConsoleLoggerConfig{
			Colorize: true,
			Writer:   a.Stderr,
		}

		// err is always nil
		consoleLogger, _ := gomolconsole.NewConsoleLogger(&consoleConfig)

		template := logTemplate
		if a.Stderr == os.Stderr {
			template = timestamp + template
		}
		// err is always nil because the template is not dynamic and I tested it at least once
		tpl, _ := gomol.NewTemplate(template)

		// err is always nil if the template is non-nil
		_ = consoleLogger.SetTemplate(tpl)

		logger := gomol.NewBase(
			func(b *gomol.Base) {
				b.SetConfig(
					&gomol.Config{
						FilenameAttr:   "filename",
						LineNumberAttr: "lineno",
						SequenceAttr:   "seq",
						MaxQueueSize:   10000,
					},
				)
			},
		)

		// err is always nil since we're not reusing objects
		_ = logger.AddLogger(consoleLogger)

		a.logger = logger

		_ = logger.InitLoggers()
	}

	return a.logger
}

func (a *App) ensureErrorChannel() {
	a.errchMu.Lock()
	defer a.errchMu.Unlock()

	if a.errch == nil {
		a.errch = make(chan error, 1)
	}
}

// Errors returns the error channel for this app.
func (a *App) Errors() <-chan error {
	a.ensureErrorChannel()
	return a.errch
}

// HandleError sends the supplied error via the Errors channel. The channel is closed after sending.
func (a *App) HandleError(err error) {
	a.ensureErrorChannel()
	a.errch <- err
	close(a.errch)
}

// LookupEnv searches the app environment variables for the specified key. If the key is found, returns a tuple of the
// value and true. If not found, returns the zero string and false.
func (a *App) LookupEnv(key string) (string, bool) {
	ch := make(chan string)

	wg := sync.WaitGroup{}
	wg.Add(len(a.Environment))

	go func() {
		for _, line := range a.Environment {
			line := line
			go func() {
				defer wg.Done()
				select {
				case <-a.Context.Done():
					return
				default:
					slice := strings.SplitN(line, "=", 2)
					if len(slice) == 2 && slice[0] == key {
						ch <- slice[1]
					}
				}
			}()
		}
	}()

	go func() {
		defer close(ch)
		wg.Wait()
	}()

	v, ok := <-ch
	return v, ok
}
