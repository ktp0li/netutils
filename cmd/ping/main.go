package main

import (
	"fmt"
	"log"
	"net"
	"ping/internal/icmp"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	flag "github.com/spf13/pflag"
)

const (
	connTimeToDeadline        = 1 * time.Minute
	timeToSleepBetweenPackets = 1 * time.Second
)

var (
	flagAddressToConnect string
	logLevel             zapcore.Level
	flagPacketsCount     int
)

func main() {
	loggerConfig := zap.NewProductionConfig()
	loggerConfig.Level.SetLevel(logLevel)

	defaultLogger := zap.Must(loggerConfig.Build())

	logger := defaultLogger.Sugar()
	logger.Level()
	defer logger.Sync()

	conn, err := net.Dial("ip4:icmp", flagAddressToConnect)
	if err != nil {
		logger.Fatalf("cannot create connection: %v", err)
	}

	conn.SetDeadline(time.Now().Add(connTimeToDeadline))
	defer conn.Close()

	fmt.Printf("PING %v\n", conn.RemoteAddr())

	if flagPacketsCount == 0 {
		packetCounter := 1
		for {
			sendAndReceiveEchoPacket(logger, conn, packetCounter)
			packetCounter += 1
		}
	} else {
		for i := 1; i <= flagPacketsCount; i += 1 {
			sendAndReceiveEchoPacket(logger, conn, i)
		}
	}

}

func init() {
	flag.IntVarP(&flagPacketsCount, "count", "c", 0, "stop after <count> replies")
	isDebug := flag.BoolP("debug", "d", false, "")
	flag.Parse()

	if len(flag.Args()) != 1 {
		log.Fatal("you must provide address to ping")
	}

	if *isDebug {
		logLevel = zap.DebugLevel
	} else {
		logLevel = zap.InfoLevel
	}

	flagAddressToConnect = flag.Arg(0)
}

func sendAndReceiveEchoPacket(logger *zap.SugaredLogger, conn net.Conn, counter int) {
	buff := make([]byte, 1024)

	icmpEchoPacket := icmp.CreateEchoPacket([]byte("heyy"))

	icmpEchoPacket.SequenceNumber = uint16(counter)
	icmpEchoRawPacket, err := icmpEchoPacket.Prepare()
	if err != nil {
		logger.Fatalf("failed to prepare icmp echo packet: %v", err)
	}

	logger.Debugf("prepared packet: %+v, binary version: %v", icmpEchoPacket, icmpEchoRawPacket)

	_, err = conn.Write(icmpEchoRawPacket)
	if err != nil {
		logger.Fatal(err)
	}

	length, err := conn.Read(buff)
	if err != nil {
		logger.Fatal(err)
	}

	logger.Debugf("read %v bytes, got packet: %v", length, buff)
	reply := icmp.ParseEchoReplyPacket(buff[:length])
	logger.Debugf("parsed package: %+v", reply)

	rttTime := time.Since(time.UnixMilli(int64(reply.Data.Timestamp)))

	fmt.Printf("%d bytes from %v: icmp_seq=%v, time=%v ms\n", len(icmpEchoRawPacket), conn.RemoteAddr(), counter, rttTime.Milliseconds())

	time.Sleep(time.Second * 1)
}
