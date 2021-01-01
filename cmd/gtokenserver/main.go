package main

/*
Copyright 2020 IKEDA Yasuyuki

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

import (
	"fmt"
	"os"

	"github.com/ikedam/gtokenserver"
	"github.com/ikedam/gtokenserver/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	pflag.StringP(
		"host",
		"h",
		"localhost",
		"Address to bind: specify 0.0.0.0 to accept remote connections especially inside docker.",
	)
	pflag.IntP("port", "p", 80, "Port to bind")
	pflag.String("log-level", "Warning", "Log level: Trace, Debug, Info, Warning, Error")
	pflag.BoolP("version", "v", false, "Show version and exit")

	pflag.Parse()
	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		log.WithError(err).Errorf("Failed to parse configurations")
		os.Exit(gtokenserver.ExitCodeInvalidConfiguration)
	}

	if viper.GetBool("version") {
		fmt.Printf("gtokenserver %v:%v\n", version, commit)
		os.Exit(0)
	}
	if err := log.SetLevelByName(viper.GetString("log-level")); err != nil {
		log.WithError(err).Errorf("Failed to configure log-level")
		os.Exit(gtokenserver.ExitCodeInvalidConfiguration)
	}

	var config gtokenserver.ServerConfig
	if err := viper.Unmarshal(&config); err != nil {
		log.WithError(err).Errorf("Failed to parse configurations")
		os.Exit(gtokenserver.ExitCodeInvalidConfiguration)
	}
	server := gtokenserver.NewServer(&config)
	if err := server.Serve(); err != nil {
		log.WithError(err).Errorf("Failed to launch server")
		os.Exit(gtokenserver.ExitCodeInvalidConfiguration)
	}
	os.Exit(0)
}
