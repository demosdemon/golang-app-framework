package app_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"

	"github.com/aphistic/gomol"
	"github.com/stretchr/testify/assert"

	"github.com/demosdemon/golang-app-framework/app"
)

func newApp(environ []string, args ...string) *app.App {
	return &app.App{
		Arguments:   args,
		Environment: environ,
		Context:     context.Background(),
		Stdin:       new(bytes.Buffer),
		Stdout:      new(bytes.Buffer),
		Stderr:      new(bytes.Buffer),
		ExitHandler: func(code int) {
			panic(fmt.Sprintf("system exit %d", code))
		},
	}
}

func TestNew(t *testing.T) {
	expected := app.App{
		Arguments:   os.Args[1:],
		Environment: os.Environ(),
		Context:     context.Background(),
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		ExitHandler: os.Exit,
	}

	p, ok := expected.LookupEnv("HOME")
	assert.NotZero(t, p)
	assert.True(t, ok)

	res := app.New()

	v, ok := res.LookupEnv("HOME")
	assert.Equal(t, p, v)
	assert.True(t, ok)
}

func TestApp_Exit(t *testing.T) {
	a := newApp(nil)

	assert.PanicsWithValue(t, "system exit 255", func() {
		a.Exit(255)
	})

	a.ExitHandler = func(int) {}

	assert.PanicsWithValue(t, "exit handler returned", func() {
		a.Exit(255)
	})

	a = newApp(nil)
	l := a.Logger()
	err := l.Warn("test")
	assert.NoError(t, err)
	assert.PanicsWithValue(t, "system exit 255", func() {
		a.Exit(255)
	})
	assert.False(t, l.IsInitialized())
	assert.Equal(
		t,
		"[\x1b[33mWARN\x1b[0m] test {\"filename\":\"base.go\",\"lineno\":447,\"seq\":1}\n",
		a.Stderr.(*bytes.Buffer).String(),
	)
}

func TestApp_Logger(t *testing.T) {
	a := newApp(nil)

	n := rand.Intn(8) + 2
	ch := make(chan *gomol.Base, n)

	wg := new(sync.WaitGroup)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			ch <- a.Logger()
		}()
	}
	wg.Wait()
	close(ch)

	l := <-ch
	for o := range ch {
		assert.Equal(t, l, o)
	}

	err := l.ShutdownLoggers()
	assert.NoError(t, err)
}

func TestApp_Errors(t *testing.T) {
	a := newApp(nil)

	n := rand.Intn(8) + 2
	ch := make(chan (<-chan error), n)

	wg := new(sync.WaitGroup)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			ch <- a.Errors()
		}()
	}
	wg.Wait()
	close(ch)

	l := <-ch
	for o := range ch {
		assert.Equal(t, l, o)
	}
}

func TestApp_HandleError(t *testing.T) {
	a := newApp(nil)
	err := errors.New("test error")
	done := make(chan struct{})
	go func() {
		a.HandleError(err)
		done <- struct{}{}
	}()

	<-done

	select {
	case x, ok := <-a.Errors():
		assert.True(t, ok)
		assert.Equal(t, err, x)
	default:
		assert.Fail(t, "channel blocked")
	}

	select {
	case x, ok := <-a.Errors():
		assert.False(t, ok)
		assert.NoError(t, x)
	default:
		assert.Fail(t, "channel blocked")
	}

	assert.Panics(t, func() {
		a.HandleError(err)
	})
}

func TestApp_LookupEnv(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	a := newApp([]string{
		"HOME=/home/test",
		"PATH=/bin",
		"TEST=true",
	})
	a.Context = ctx

	value, ok := a.LookupEnv("HOME")
	assert.True(t, ok)
	assert.Equal(t, "/home/test", value)

	value, ok = a.LookupEnv("PATH")
	assert.True(t, ok)
	assert.Equal(t, "/bin", value)

	value, ok = a.LookupEnv("FOOBAR")
	assert.False(t, ok)
	assert.Zero(t, value)

	cancel()

	value, ok = a.LookupEnv("HOME")
	assert.False(t, ok)
	assert.Zero(t, value)
}

func BenchmarkApp_LookupEnv(b *testing.B) {
	environ := make([]string, 0, b.N)
	environ = append(environ, "HOME=/run/test")
	for i := 1; i < b.N; i++ {
		environ = append(environ, fmt.Sprintf("TEST_%06d=%d", i, i))
	}

	rand.Shuffle(b.N, func(i, j int) { environ[i], environ[j] = environ[j], environ[i] })

	a := newApp(environ, "test")
	b.ResetTimer()
	v, ok := a.LookupEnv("HOME")
	assert.True(b, ok)
	assert.Equal(b, "/run/test", v)
}
