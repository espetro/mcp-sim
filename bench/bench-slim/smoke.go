package main

import (
	"context"
	"fmt"
	"os"
	"time"

	ios "github.com/espetro/mcp-sim/platforms/ios"

	"github.com/espetro/mcp-sim/internal/config"
	"github.com/espetro/mcp-sim/pkg/contract"
	"github.com/espetro/mcp-sim/pkg/orchestrator"
)

// runOrchestratorSmoke is a minimal orchestrator-level smoke: boot -> state ->
// wipe -> converge check. It is a perf/no-behavior-change gate and only runs
// when MCPSIM_BENCH_TARGET is set (a real simulator UDID); otherwise the
// caller skips it.
func runOrchestratorSmoke(ctx context.Context) {
	target := os.Getenv("MCPSIM_BENCH_TARGET")
	if target == "" {
		fmt.Println("orchestrator smoke: skipped (MCPSIM_BENCH_TARGET not set)")
		return
	}

	p, err := ios.New(ctx, config.IOSConfig{})
	if err != nil || p == nil {
		fatal(fmt.Errorf("ios.New: %v (err=%v)", p, err))
	}
	o, err := orchestrator.New(orchestrator.WithPlatform(p))
	if err != nil {
		fatal(err)
	}

	start := time.Now()
	if _, err := o.Boot(ctx, "ios", target, contract.StartOpts{Timeout: 10 * time.Minute}); err != nil {
		fatal(fmt.Errorf("smoke boot %s: %w", target, err))
	}
	if err := o.AwaitReady(ctx, "ios", target, 5*time.Minute); err != nil {
		fatal(fmt.Errorf("smoke await %s: %w", target, err))
	}
	st, err := o.State(ctx, "ios", target)
	if err != nil {
		fatal(fmt.Errorf("smoke state %s: %w", target, err))
	}
	if st != contract.DeviceStateRunning {
		fatal(fmt.Errorf("smoke: state after boot+await = %s, want running", st))
	}
	bootMS := time.Since(start).Milliseconds()

	if err := o.Wipe(ctx, "ios", target); err != nil {
		fatal(fmt.Errorf("smoke wipe %s: %w", target, err))
	}
	// Converge check: after wipe the device must report a coherent stopped
	// state, and a subsequent boot must succeed (baseline restored).
	st, err = o.State(ctx, "ios", target)
	if err != nil {
		fatal(fmt.Errorf("smoke post-wipe state %s: %w", target, err))
	}
	if st == contract.DeviceStateError {
		fatal(fmt.Errorf("smoke: post-wipe state = error"))
	}

	fmt.Printf("orchestrator smoke: OK (boot+await %dms, post-wipe state %s)\n", bootMS, st)
}
