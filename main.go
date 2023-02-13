package main

import (
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/mpetavy/common"
)

var (
	conn        = flag.String("c", "", "<connection:port> to send to")
	filename    = flag.String("f", "", "file to send")
	readtimeout = flag.Int("rt", 3000, "timeout in seconds for reading ACK")
	looptimeout = flag.Int("lt", 1000, "timeout in seconds to wait between loops")
	loopCount   = flag.Int("lc", 1, "count loop")
	plain       = flag.Bool("p", false, "no MLLP framing, just send")
	useTls      = flag.Bool("tls", false, "Use TLS")
	useACK      = flag.Bool("ack", false, "Use ACK")
)

func init() {
	common.Init("hl7send", "1.0.4", "", "", "2018", "Send HL7/TXT files", "mpetavy", fmt.Sprintf("https://github.com/mpetavy/%s", common.Title()), common.APACHE, nil, nil, nil, run, 0)
}

func send(connection common.EndpointConnection, filename string) error {
	sendBuffer, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	if !*plain {
		common.Debug("Add MLLP framing")

		sendBuf := bytes.Buffer{}
		sendBuf.Write([]byte{0xb})
		sendBuf.Write(sendBuffer)
		sendBuf.Write([]byte{0x1c, 0xd})

		sendBuffer = sendBuf.Bytes()
	}

	common.Debug("send bytes")
	n, err := connection.Write(sendBuffer)
	if err != nil {
		return err
	}
	common.Debug("File: %s bytes written: %d", filename, n)

	if *useACK {
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

		if !*plain {
			receiveBuffer = receiveBuffer[1 : n-2]
		}

		fmt.Printf("%+v\n", string(receiveBuffer))
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
		fw, err := common.NewFilewalker(*filename, true, false, func(path string, f os.FileInfo) error {
			if f.IsDir() {
				return nil
			}

			if *loopCount > 1 {
				fmt.Printf("Loop: #%d: %s\n", c, path)
			}

			err := send(connection, path)
			if common.Error(err) {
				return err
			}

			if *loopCount > 1 && (c+1) < *loopCount {
				time.Sleep(time.Millisecond * time.Duration(*looptimeout))
			}

			return nil
		})
		if common.Error(err) {
			return err
		}

		err = fw.Run()
		if common.Error(err) {
			return err
		}
	}

	return nil
}

func main() {
	common.Run([]string{"f"})
}
