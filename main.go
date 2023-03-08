package main

import (
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mpetavy/common"
)

var (
	conn        = flag.String("c", "", "<connection:port> to send to")
	filename    = flag.String("f", "", "file to send")
	readtimeout = flag.Int("rt", 3000, "timeout in seconds for reading ACK")
	looptimeout = flag.Int("lt", 1000, "timeout in seconds to wait between loops")
	loopCount   = flag.Int("lc", 1, "count loop")
	useTls      = flag.Bool("tls", false, "Use TLS")
	useACK      = flag.Bool("ack", false, "Use ACK")
	recursive   = flag.Bool("r", false, "Recursive directory scan")

	HL7Start = []byte{0xb}
	HL7End   = []byte{0x1c, 0xd}
)

func init() {
	common.Init("hl7send", "1.0.4", "", "", "2018", "Send HL7/TXT files", "mpetavy", fmt.Sprintf("https://github.com/mpetavy/%s", common.Title()), common.APACHE, nil, nil, nil, run, 0)
}

func send(connection common.EndpointConnection, filename string) error {
	common.Info(strings.Repeat("-", 40))

	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	common.Info("Send file %s:", filename)

	buf := bytes.Buffer{}
	buf.Write(HL7Start)
	buf.Write(content)
	buf.Write(HL7End)

	common.Info("")
	common.Info(common.PrintBytes(buf.Bytes(), true))
	common.Info("")

	n, err := connection.Write(buf.Bytes())
	if err != nil {
		return err
	}

	common.Info("Bytes sent: %d", n)

	if *useACK {
		common.Info("")
		common.Info("Wait for ACK...")

		if *readtimeout > 0 {
			common.Debug("SetReadDeadline: %v", *readtimeout)

			err := connection.SetReadDeadline(time.Now().Add(common.MillisecondToDuration(*readtimeout)))
			if err != nil {
				return err
			}
		}

		receiveBuffer := make([]byte, 1024*1024)

		n, err = connection.Read(receiveBuffer)
		if err != nil {
			return err
		}

		receiveBuffer = receiveBuffer[:n]

		common.Info("")
		common.Info("ACK received:")

		common.Info(common.PrintBytes(receiveBuffer, true))
	}

	return nil
}

func run() error {
	var tlsConfig *tls.Config
	var err error

	if *useTls {
		tlsConfig, err = common.NewTlsConfigFromFlags()
		if common.Error(err) {
			return err
		}
	}

	ep, connector, err := common.NewEndpoint(*conn, true, tlsConfig)
	if common.Error(err) {
		return err
	}

	err = ep.Start()
	if common.Error(err) {
		return err
	}

	defer func() {
		common.Error(ep.Stop())
	}()

	connection, err := connector()
	if common.Error(err) {
		return err
	}

	defer func() {
		common.DebugError(connection.Close())
	}()

	for c := 0; c < *loopCount; c++ {
		err := common.WalkFiles(*filename, *recursive, false, func(path string, f os.FileInfo) error {
			if f.IsDir() {
				return nil
			}

			if *loopCount > 1 {
				common.Info("Loop: #%d: %s\n", c, path)
			}

			err := send(connection, path)
			if common.Error(err) {
				return err
			}

			if *loopCount > 1 && (c+1) < *loopCount {
				common.Sleep(time.Millisecond * time.Duration(*looptimeout))
			}

			return nil
		})
		if common.Error(err) {
			return err
		}
	}

	return nil
}

func main() {
	common.Run([]string{"f"})
}
