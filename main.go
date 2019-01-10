package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
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

	openKubeConfig := func(cfgpath string) (*rest.Config, error) {
		if cfgpath == "" {
			cfgpath = path.Join(homedir.HomeDir(), ".kube", "config")
		}

		if _, err := os.Stat(cfgpath); err == nil {
			cfg, err := clientcmd.BuildConfigFromFlags("", cfgpath)
			if err != nil {
				return nil, errors.Wrap(err, cfgpath)
			}
			return cfg, nil
		}

		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, errors.Wrap(err, cfgpath+" fallback in cluster")
		}
		return cfg, nil
	}

	cmd.Run = func(cmd *cobra.Command, args []string) {
		cfg, _ := openKubeConfig(kubeCfg)
		metc, _ := metricsclient.NewForConfig(cfg)

		list, err := metc.MetricsV1beta1().NodeMetricses().List(metav1.ListOptions{})
		exitOnErr(err)
		data, _ := json.MarshalIndent(list, "", "    ")
		fmt.Println(string(data))

		list2, err := metc.MetricsV1beta1().PodMetricses("kube-system").List(metav1.ListOptions{})
		exitOnErr(err)
		data, _ = json.MarshalIndent(list2, "", "    ")
		fmt.Println(string(data))
	}

	return cmd
}

func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start <dc-name>",
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
			runner, err := task.NewRunner(*cfgpath, *ns, *host)
			exitOnErr(err)

			exitOnErr(runner.CreateTasks(args[0], args[1:]...))
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "job <name> <images>",
		Short: "create job task",
		Long:  "create a new job task with your images a new job task with your images",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			runner, err := task.NewRunner(*cfgpath, *ns, *host)
			exitOnErr(err)

			exitOnErr(runner.CreateJobs(args[0], "cron" /*FIXME*/, args[1:]...))
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

			runner, err := task.NewRunner(*cfgpath, *ns, *host)
			exitOnErr(err)

			exitOnErr(runner.UpdateTask(args[0], args[1], uint32(replicas), 80, 80))
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "delete <name>",
		Short: "delete exist task",
		Long:  "delete a exist task",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runner, err := task.NewRunner(*cfgpath, *ns, *host)
			exitOnErr(err)

			exitOnErr(runner.CancelTask(args[0]))
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "list tasks",
		Long:  "list all tasks running",
		Run: func(cmd *cobra.Command, args []string) {
			runner, err := task.NewRunner(*cfgpath, *ns, *host)
			exitOnErr(err)

			tasks, err := runner.ListTask()
			exitOnErr(err)

			for _, task := range tasks {
				fmt.Println(task)
			}
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
			runner, err := task.NewRunner(*cfgpath, *ns, *host)
			exitOnErr(err)

			metering, err := runner.Metering()
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
