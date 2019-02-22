package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Ankr-network/dccn-daemon/daemon"
	"github.com/Ankr-network/dccn-daemon/task"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/types"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/homedir"
)

var (
	version string
	commit  string
	date    string

	kubeCfg = filepath.Join(homedir.HomeDir(), ".kube", "config")
)

func main() {
	rootCmd := &cobra.Command{
		Use:   os.Args[0] + " <command>",
		Short: os.Args[0] + " is a ankr dccn daemon tool",
	}
	{
		verbose := rootCmd.PersistentFlags().Uint16P("verbose", "v", 0, "log verbose level")

		rootCmd.PersistentPreRun = func(*cobra.Command, []string) {
			// Compatible with flag for github.com/golang/glog
			flag.Set("logtostderr", "true")
			flag.Set("v", strconv.Itoa(int(*verbose)))
			flag.Parse()
		}
	}

	rootCmd.AddCommand(versionCmd())
	rootCmd.AddCommand(taskCmd())
	rootCmd.AddCommand(startCmd())
	rootCmd.AddCommand(blockchainCmd())
	rootCmd.AddCommand(metricCmd())
	rootCmd.Execute()
}

func versionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "print version info",
	}

	cmd.Run = func(*cobra.Command, []string) {
		fmt.Println("Version:", version)
		fmt.Println("Commit:", commit)
		fmt.Println("Compile Date:", date)
	}
	return cmd
}

func metricCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metric",
		Short: "metric daemon server",
		Long:  "metric a long running server to handle ankr-hub requests",
	}

	ns := cmd.Flags().StringP("namespace", "n", apiv1.NamespaceDefault, "kubernetes namespace")
	host := cmd.Flags().String("ingress-host", "localhost", "kubernetes ingress host")
	cfgpath := cmd.Flags().String("k8s-cfg", kubeCfg, "kubernetes config")

	cmd.Run = func(cmd *cobra.Command, args []string) {
		client, err := task.NewTasker("", *cfgpath, *ns, *host)
		exitOnErr(err)

		metrics, err := client.Metrics()
		exitOnErr(err)

		data, err := json.MarshalIndent(metrics, "", "    ")
		exitOnErr(err)

		fmt.Printf("%s\n", data)
	}

	return cmd
}

func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start <dc-c>",
		Short: "start daemon server",
		Long:  "start a long running server to handle ankr-hub requests",
	}

	server := cmd.Flags().StringP("hub-server", "s", "hub.ankr.network", "ankr hub server address")
	port := cmd.Flags().Uint32P("port", "p", 50051, "ankr hub port number")
	ns := cmd.Flags().StringP("namespace", "n", apiv1.NamespaceDefault, "kubernetes namespace")
	host := cmd.Flags().String("ingress-host", "localhost", "kubernetes ingress host")
	cfgpath := cmd.Flags().String("k8s-cfg", kubeCfg, "kubernetes config")
	tendermintServer := cmd.PersistentFlags().StringP("tendermint-server", "S", "127.0.0.1", "special tendermint grpc server")
	tendermintPort := cmd.PersistentFlags().Uint32P("tendermint-port", "P", 26657, "special tendermint grpc port")
	tendermintWsEndpoint := cmd.PersistentFlags().StringP("tendermint-websocket-endpoint", "W", "/websocket", "special tendermint websocket endpoint")

	cmd.PreRun = func(*cobra.Command, []string) {
		glog.Infoln("version:", version, "commit:", commit, "date:", date)
		glog.Infoln("Starting with cmd:", os.Args)
	}

	cmd.Args = func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("dc name must set")
		}
		if *server == "" {
			return errors.New("server address must set")
		}
		if *port <= 0 || *port >= 65536 {
			return errors.New("ports not correct")
		}
		return nil
	}

	cmd.Run = func(cmd *cobra.Command, args []string) {
		glog.Infof("Starting, hub: %s:%d", *server, *port)
		glog.Fatalln(daemon.ServeTask(*cfgpath, *ns, *host,
			fmt.Sprintf("%s:%d", *server, *port), args[0],
			fmt.Sprintf("%s:%d", *tendermintServer, *tendermintPort), *tendermintWsEndpoint))
	}

	return cmd
}

func taskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "run single task",
		Long:  "task is for run a single task in command line",
	}

	ns := cmd.PersistentFlags().StringP("namespace", "n", apiv1.NamespaceDefault, "kubernetes namespace")
	host := cmd.PersistentFlags().String("ingress-host", "localhost", "kubernetes ingress host")
	cfgpath := cmd.PersistentFlags().String("k8s-cfg", kubeCfg, "kubernetes config")

	cmd.AddCommand(&cobra.Command{
		Use:   "create <name> <images>",
		Short: "create deploy task",
		Long:  "create a new deploy task with your images",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			client, err := task.NewTasker("", *cfgpath, *ns, *host)
			exitOnErr(err)

			exitOnErr(client.CreateTasks(args[0], args[1:]...))
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "update <name> <images> <replicas>",
		Short: "update exist task",
		Long:  "update a exist task with your options",
		Args:  cobra.MinimumNArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			replicas, err := strconv.ParseUint(args[2], 10, 32)
			exitOnErr(err)

			client, err := task.NewTasker("", *cfgpath, *ns, *host)
			exitOnErr(err)

			exitOnErr(client.UpdateTask(args[0], args[1], uint32(replicas), 80, 80))
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "delete <name>",
		Short: "delete exist task",
		Long:  "delete a exist task",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client, err := task.NewTasker("", *cfgpath, *ns, *host)
			exitOnErr(err)

			exitOnErr(client.CancelTask(args[0]))
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "list tasks",
		Long:  "list all tasks running",
		Run: func(cmd *cobra.Command, args []string) {
			client, err := task.NewTasker("", *cfgpath, *ns, *host)
			exitOnErr(err)

			tasks, err := client.ListTask()
			exitOnErr(err)

			for _, task := range tasks {
				fmt.Println(task)
			}
		},
	})

	return cmd
}

func jobCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "job",
		Short: "run single job",
		Long:  "job is for run a single job in command line",
	}

	ns := cmd.PersistentFlags().StringP("namespace", "n", apiv1.NamespaceDefault, "kubernetes namespace")
	host := cmd.PersistentFlags().String("ingress-host", "localhost", "kubernetes ingress host")
	cfgpath := cmd.PersistentFlags().String("k8s-cfg", kubeCfg, "kubernetes config")

	cmd.AddCommand(&cobra.Command{
		Use:   "create <name> <images> [crontab]",
		Short: "create (cron)job",
		Long:  "create a new (cron)job with your images",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			client, err := task.NewTasker("", *cfgpath, *ns, *host)
			exitOnErr(err)

			crontab := ""
			if len(args) >= 3 {
				crontab = args[2]
			}
			exitOnErr(client.CreateJobs(args[0], crontab, args[1:]...))
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "delete <name> [crontab]",
		Short: "delete exist (cron)job",
		Long:  "delete a exist (cron)job",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client, err := task.NewTasker("", *cfgpath, *ns, *host)
			exitOnErr(err)

			crontab := ""
			if len(args) >= 3 {
				crontab = args[2]
			}
			exitOnErr(client.CancelJob(args[0], crontab))
		},
	})

	return cmd
}

func blockchainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bc",
		Short: "blockchain service",
		Long:  "send or query message on blockchain",
	}

	ns := cmd.PersistentFlags().StringP("namespace", "n", apiv1.NamespaceDefault, "kubernetes namespace")
	host := cmd.PersistentFlags().String("ingress-host", "localhost", "kubernetes ingress host")
	cfgpath := cmd.PersistentFlags().String("k8s-cfg", kubeCfg, "kubernetes config")

	server := cmd.PersistentFlags().StringP("tendermint-server", "s", "127.0.0.1", "special tendermint grpc server")
	port := cmd.PersistentFlags().Uint32P("tendermint-port", "p", 26657, "special tendermint grpc port")
	wsEndpoint := cmd.PersistentFlags().StringP("tendermint-websocket-endpoint", "w", "/websocket", "special tendermint websocket endpoint")

	cmd.AddCommand(&cobra.Command{
		Use:   "metering <dc-name>",
		Short: "store metering info into tendermint",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// collect data
			clientTask, err := task.NewTasker("", *cfgpath, *ns, *host)
			exitOnErr(err)

			metering, err := clientTask.Metering()
			exitOnErr(err)

			data, err := json.Marshal(metering)
			exitOnErr(err)

			// write into tendermint
			c := client.NewHTTP(fmt.Sprintf("tcp://%s:%d", *server, *port), *wsEndpoint)

			_, err = c.BroadcastTxCommit(types.Tx(
				fmt.Sprintf("%s_%s=%s", args[0], *ns, data)))
			exitOnErr(err)

			// query
			resp, err := c.ABCIQuery(*wsEndpoint, cmn.HexBytes(fmt.Sprintf("%s_%s", args[0], *ns)))
			exitOnErr(err)

			fmt.Printf("%s\n", resp.Response.GetValue())
		},
	})

	return cmd
}

func exitOnErr(err error, a ...interface{}) {
	if err == nil {
		return
	}

	if len(a) == 0 {
		fmt.Printf("FAIL: %+v\n", err)
	} else {
		fmt.Println(a...)
	}
	os.Exit(1)
}
