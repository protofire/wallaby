package main

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/lotus/api"
	lcli "github.com/filecoin-project/lotus/cli"
	"github.com/filecoin-project/lotus/lib/tablewriter"
)

var dagstoreCmd = &cli.Command{
	Name:  "dagstore",
	Usage: "Manage the dagstore on the markets subsystem",
	Subcommands: []*cli.Command{
		dagstoreListShardsCmd,
		dagstoreInitializeShardCmd,
		dagstoreRecoverShardCmd,
		dagstoreInitializeAllCmd,
		dagstoreGcCmd,
	},
}

var dagstoreListShardsCmd = &cli.Command{
	Name:  "list-shards",
	Usage: "List all shards known to the dagstore, with their current status",
	Action: func(cctx *cli.Context) error {
		marketsApi, closer, err := lcli.GetMarketsAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := lcli.ReqContext(cctx)

		shards, err := marketsApi.DagstoreListShards(ctx)
		if err != nil {
			return err
		}

		if len(shards) == 0 {
			return nil
		}

		tw := tablewriter.New(
			tablewriter.Col("Key"),
			tablewriter.Col("State"),
			tablewriter.Col("Error"),
		)

		colors := map[string]color.Attribute{
			"ShardStateAvailable": color.FgGreen,
			"ShardStateServing":   color.FgBlue,
			"ShardStateErrored":   color.FgRed,
			"ShardStateNew":       color.FgYellow,
		}

		for _, s := range shards {
			m := map[string]interface{}{
				"Key": s.Key,
				"State": func() string {
					if c, ok := colors[s.State]; ok {
						return color.New(c).Sprint(s.State)
					}
					return s.State
				}(),
				"Error": s.Error,
			}
			tw.Write(m)
		}

		return tw.Flush(os.Stdout)
	},
}

var dagstoreInitializeShardCmd = &cli.Command{
	Name:      "initialize-shard",
	ArgsUsage: "[key]",
	Usage:     "Initialize the specified shard",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return fmt.Errorf("must provide a single shard key")
		}

		marketsApi, closer, err := lcli.GetMarketsAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := lcli.ReqContext(cctx)

		return marketsApi.DagstoreInitializeShard(ctx, cctx.Args().First())
	},
}

var dagstoreRecoverShardCmd = &cli.Command{
	Name:      "recover-shard",
	ArgsUsage: "[key]",
	Usage:     "Attempt to recover a shard in errored state",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return fmt.Errorf("must provide a single shard key")
		}

		marketsApi, closer, err := lcli.GetMarketsAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := lcli.ReqContext(cctx)

		return marketsApi.DagstoreRecoverShard(ctx, cctx.Args().First())
	},
}

var dagstoreInitializeAllCmd = &cli.Command{
	Name:  "initialize-all",
	Usage: "Initialize all uninitialized shards, streaming results as they're produced",
	Flags: []cli.Flag{
		&cli.UintFlag{
			Name:     "concurrency",
			Usage:    "maximum shards to initialize concurrently at a time; use 0 for unlimited",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) error {
		concurrency := cctx.Uint("concurrency")

		marketsApi, closer, err := lcli.GetMarketsAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := lcli.ReqContext(cctx)

		params := api.DagstoreInitializeAllParams{
			MaxConcurrency: int(concurrency),
		}

		ch, err := marketsApi.DagstoreInitializeAll(ctx, params)
		if err != nil {
			return err
		}

		for {
			select {
			case evt, ok := <-ch:
				if !ok {
					return nil
				}
				_, _ = fmt.Fprintf(os.Stdout, color.New(color.BgHiBlack).Sprintf("(%d/%d)", evt.Current, evt.Total))
				_, _ = fmt.Fprintf(os.Stdout, " ")
				if evt.Event == "start" {
					_, _ = fmt.Fprintln(os.Stdout, evt.Key, color.New(color.Reset).Sprint("STARTING"))
				} else {
					if evt.Success {
						_, _ = fmt.Fprintln(os.Stdout, evt.Key, color.New(color.FgGreen).Sprint("SUCCESS"))
					} else {
						_, _ = fmt.Fprintln(os.Stdout, evt.Key, color.New(color.FgRed).Sprint("ERROR"), evt.Error)
					}
				}

			case <-ctx.Done():
				return fmt.Errorf("aborted")
			}
		}
	},
}

var dagstoreGcCmd = &cli.Command{
	Name:  "gc",
	Usage: "Garbage collect the dagstore",

	Action: func(cctx *cli.Context) error {
		marketsApi, closer, err := lcli.GetMarketsAPI(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := lcli.ReqContext(cctx)

		collected, err := marketsApi.DagstoreGC(ctx)
		if err != nil {
			return err
		}

		if len(collected) == 0 {
			_, _ = fmt.Fprintln(os.Stdout, "no shards collected")
			return nil
		}

		for _, e := range collected {
			if e.Error == "" {
				_, _ = fmt.Fprintln(os.Stdout, e.Key, color.New(color.FgGreen).Sprint("SUCCESS"))
			} else {
				_, _ = fmt.Fprintln(os.Stdout, e.Key, color.New(color.FgRed).Sprint("ERROR"), e.Error)
			}
		}

		return nil
	},
}