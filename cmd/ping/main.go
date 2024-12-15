package main

import (
	"fmt"
	"log"
	"net"
	"ping/internal/icmp"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/ipv4"

	"github.com/pkg/errors"
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
	flagIsPrivileged     bool
)

var (
	addressToBind = net.ParseIP("0.0.0.0")
)

func main() {
	// --- init logger
	loggerConfig := zap.NewProductionConfig()
	loggerConfig.Level.SetLevel(logLevel)

	defaultLogger := zap.Must(loggerConfig.Build())

	logger := defaultLogger.Sugar()
	logger.Level()
	defer logger.Sync()
	// ---

	// parse address to connect
	address, err := getIPv4Addr(flagAddressToConnect)
	if err != nil {
		logger.Fatalf("cannot get ipv4 address: %v", err)
	}

	var conn *ipv4.PacketConn
	if flagIsPrivileged {
		conn, err = icmp.NewPrivilegedIPv4Connection(addressToBind.String())
	} else {
		conn, err = icmp.NewUnprivilegedIPv4Connection([4]byte(addressToBind))
	}

	if err != nil {
		logger.Fatalf("cannot create new connection: %v", err)
	}

	conn.SetControlMessage(ipv4.FlagTTL, true)
	conn.SetDeadline(time.Now().Add(connTimeToDeadline))
	defer conn.Close()

	icmpExamplePacket := icmp.CreateEchoPacket([]byte("heyy"))
	packetSize := icmpExamplePacket.Length() + ipv4.HeaderLen

	fmt.Printf("PING %v (%v) with %d(%d) bytes of data\n", flagAddressToConnect, address, icmpExamplePacket.Data.Length(), packetSize)

	icmpDestAddr := net.UDPAddr{IP: address}

	if flagPacketsCount == 0 {
		packetCounter := 1
		for {
			sendAndReceiveEchoPacket(logger, conn, &icmpDestAddr, packetCounter)
			packetCounter += 1
		}
	} else {
		for i := 1; i <= flagPacketsCount; i += 1 {
			sendAndReceiveEchoPacket(logger, conn, &icmpDestAddr, i)
		}
	}

}

func init() {
	flag.IntVarP(&flagPacketsCount, "count", "c", 0, "stop after <count> replies")
	flag.BoolVar(&flagIsPrivileged, "privileged", false, "run with ip:icmp socket instead udp")
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

func sendAndReceiveEchoPacket(logger *zap.SugaredLogger, conn *ipv4.PacketConn, addr *net.UDPAddr, seqNumber int) {
	buff := make([]byte, 1024)

	icmpEchoPacket := icmp.CreateEchoPacket([]byte("heyy"))

	icmpEchoPacket.SequenceNumber = uint16(seqNumber)
	icmpEchoRawPacket, err := icmpEchoPacket.Prepare()
	if err != nil {
		logger.Fatalf("failed to prepare icmp echo packet: %v", err)
	}

	logger.Debugf("prepared packet: %+v, binary version: %v", icmpEchoPacket, icmpEchoRawPacket)

	_, err = conn.WriteTo(icmpEchoRawPacket, nil, addr)
	if err != nil {
		logger.Fatal(err)
	}

	length, cm, _, err := conn.ReadFrom(buff)
	if err != nil {
		logger.Fatal(err)
	}

	var replyTTL int
	if cm == nil {
		replyTTL = -1
	} else {
		replyTTL = cm.TTL
	}

	logger.Debugf("read %v bytes, got packet: %v", length, buff)

	reply := icmp.ParseEchoReplyPacket(buff[:length])
	logger.Debugf("parsed package: %+v", reply)

	rttTime := time.Since(time.UnixMilli(int64(reply.Data.Timestamp)))

	fmt.Printf("%d bytes from %v: icmp_seq=%d ttl=%d time=%d ms\n", len(icmpEchoRawPacket), addr.IP, seqNumber, replyTTL, rttTime.Milliseconds())

	time.Sleep(timeToSleepBetweenPackets)
}

func getIPv4Addr(addr string) (net.IP, error) {
	if ipv4Addr := net.ParseIP(addr).To4(); ipv4Addr != nil {
		return ipv4Addr, nil
	}

	resolvedAddr, err := net.LookupIP(addr)
	if err != nil {
		return nil, errors.Wrap(err, "cannot resolve address")
	}

	for _, resolvedIP := range resolvedAddr {
		if ipv4Addr := resolvedIP.To4(); ipv4Addr != nil {
			return ipv4Addr, nil
		}
	}

	return nil, fmt.Errorf("cannot find valid ipv4 address for this domain")
}
