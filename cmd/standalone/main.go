package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/sirupsen/logrus"

	"github.com/docker/stacks/pkg/controller/standalone"
)

var debugFlag = cli.BoolFlag{
	Name:  "debug",
	Usage: "Enable debug logging",
}

var socketFlag = cli.StringFlag{
	Name:  "docker-socket",
	Usage: "Path to the Docker socket (default: /var/run/docker.sock)",
	Value: "/var/run/docker.sock",
}

var portFlag = cli.IntFlag{
	Name:  "port",
	Usage: "Port on which to expose the stacks API (default: 2375)",
	Value: 2375,
}

var cmdServer = cli.Command{
	Name:   "server",
	Usage:  "Starts the Standalone Stacks API server and reconciler",
	Action: RunStandaloneServer,
	Flags: []cli.Flag{
		debugFlag,
		socketFlag,
		portFlag,
	},
}

// RunStandaloneServer parses CLI arguments and runs the StandaloneServer
// method from the standalone package.
func RunStandaloneServer(c *cli.Context) error {
	s, err := standalone.CreateServer(standalone.ServerOptions{
		Debug:            c.Bool("debug"),
		DockerSocketPath: c.String("docker-socket"),
		ServerPort:       c.Int("port"),
	})
	if err != nil {
		return err
	}
	return s.RunServer()
}

func main() {
	app := cli.NewApp()
	app.Name = "Stacks Standalone Controller"
	app.Usage = "Docker Stacks Standalone Controller"
	app.Commands = []cli.Command{
		cmdServer,
	}

	logrus.SetFormatter(&logrus.JSONFormatter{})
	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
		os.Exit(1)
	}
}
