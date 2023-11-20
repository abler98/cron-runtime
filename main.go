package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"os/exec"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	debug       = flag.Bool("debug", false, "Debug info")
	once        = flag.Bool("once", false, "Run once and exit")
	killTimeout = flag.Int("kill-timeout", 0, "Kill timeout in seconds")
)

func main() {
	parseFlags()

	log.Logger = log.Output(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.TimeFormat = time.DateTime
	}))

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	programName := flag.Arg(2)
	programArgs := flag.Args()[3:]

	c := cron.New(cron.WithParser(cron.NewParser(
		cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)))

	var sig os.Signal
	terminate := make(chan struct{})
	stop := make(chan struct{})
	kill := make(chan struct{})

	var ran atomic.Bool

	_, err := c.AddFunc(flag.Arg(0), func() {
		if *once {
			if !ran.CompareAndSwap(false, true) {
				return
			}
			defer close(stop)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		cmd := exec.CommandContext(ctx, programName, programArgs...)

		if *debug {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}

		log.Info().Msgf("CMD: %s", cmd)

		err := cmd.Start()
		if err != nil {
			log.Err(err).Msg("failed to start process")
			return
		}

		pid := cmd.Process.Pid

		log.Debug().Int("pid", pid).Msg("process starting...")
		defer log.Debug().Int("pid", pid).Msg("process finished")

		wait := make(chan struct{})

		go func() {
			defer close(wait)
			err := cmd.Wait()
			if err != nil {
				log.Err(err).Int("pid", pid).Msg("process exited with error")
			}
		}()

		select {
		case <-wait:
		case <-terminate:
			log.Debug().Int("pid", pid).Msg("process terminating...")
			err := cmd.Process.Signal(os.Interrupt)
			if err != nil {
				log.Err(err).Int("pid", pid).Msg("failed to send interrupt signal")
			}
			select {
			case <-wait:
			case <-kill:
				log.Warn().Int("pid", pid).Msg("process killed")
			}
		}
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to add cron job")
	}

	c.Start()

	s := make(chan os.Signal)
	signal.Notify(s, os.Interrupt, syscall.SIGTERM)

	select {
	case sig = <-s:
		log.Info().Msgf("received %s signal, stopping...", sig)
		close(terminate)
	case <-stop:
		log.Info().Msg("stopping...")
	}

	cronCtx := c.Stop()

	if *killTimeout > 0 {
		killTimer := time.NewTimer(time.Duration(*killTimeout) * time.Second)
		select {
		case <-cronCtx.Done():
			log.Info().Msg("cron stopped")
		case <-killTimer.C:
			log.Warn().Msg("killing...")
			close(kill)
			<-cronCtx.Done()
			log.Info().Msg("cron stopped")
		}
	} else {
		<-cronCtx.Done()
		log.Info().Msg("cron stopped")
	}
}

func parseFlags() {
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() < 3 {
		usageError("invalid number of arguments")
	}

	if flag.Arg(1) != "--" {
		usageError("invalid arguments")
	}
}

func usage() {
	fmt.Printf("Usage: cron-runtime [flags] <expression> -- program [args...]\n")
	flag.CommandLine.PrintDefaults()
}

func usageError(msg string) {
	fmt.Printf("%s\n", msg)
	usage()
	os.Exit(2)
}
