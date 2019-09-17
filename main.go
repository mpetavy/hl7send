package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"time"

	"github.com/mpetavy/common"
)

var (
	conn        = flag.String("c", "localhost:9090", "<connection:port> to send to")
	filename    = flag.String("f", "", "file to send")
	readtimeout = flag.Int("rt", 3000, "timeout in seconds for reading ACK")
	looptimeout = flag.Int("wt", 1000, "timeout in seconds to wait between loops")
	loop        = flag.Int("l", 1, "count loop")
	plain       = flag.Bool("p", false, "no MLLP framing, just send")
)

func init() {
	common.Init("hl7send", "1.0.4", "2018", "Send HL7/TXT files", "mpetavy", common.APACHE, "https://github.com/mpetavy/hl7send", false, nil, nil, run, 0)
}

func run() error {
	b, err := common.FileExists(*filename)
	if err != nil {
		return err
	}

	if !b {
		common.Error(fmt.Errorf("unknown file: %s", *filename))
	}

	conn, err := net.Dial("tcp", *conn)
	if err != nil {
		return err
	}
	defer func() {
		common.DebugError(conn.Close())
	}()

	for c := 0; c < *loop; c++ {
		if *loop > 1 {
			fmt.Printf("Loop: #%d\n", c)
		}

		common.Debug("Read bytes")
		sendBuffer, err := ioutil.ReadFile(*filename)
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
		n, err := conn.Write(sendBuffer)
		if err != nil {
			return err
		}
		common.Debug("Bytes written: %d", n)

		if *readtimeout > 0 {
			common.Debug("SetReadDeadline: %v", *readtimeout)
			err := conn.SetReadDeadline(time.Now().Add(common.MsecToDuration(*readtimeout)))
			if err != nil {
				return err
			}
		}

		receiveBuffer := make([]byte, 1024*1024)

		n, err = conn.Read(receiveBuffer)
		if err != nil {
			return err
		}

		if !*plain {
			receiveBuffer = receiveBuffer[1 : n-2]
		}

		s := string(receiveBuffer)
		s = common.ConvertToOSspecificLF(s)

		fmt.Printf("%s\n", s)

		if *loop > 1 && (c+1) < *loop {
			time.Sleep(time.Millisecond * time.Duration(*looptimeout))

			continue
		}

		break
	}

	return nil
}

func main() {
	defer common.Done()

	common.Run([]string{"f"})
}
