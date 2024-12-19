package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"os/signal"
	"ping/internal/icmp"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/ipv4"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
)

const (
	connTimeToDeadline        = 10 * time.Second
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
	destAddress, err := getIPv4Addr(flagAddressToConnect)
	if err != nil {
		logger.Fatalf("cannot get ipv4 address: %v", err)
	}

	ctx, cancelCtx := context.WithCancel(context.Background())

	// init connection
	var conn *ipv4.PacketConn
	if flagIsPrivileged {
		conn, err = icmp.NewPrivilegedIPv4Connection(addressToBind.String())
	} else {
		conn, err = icmp.NewUnprivilegedIPv4Connection(addressToBind)
	}
	if err != nil {
		logger.Fatalf("cannot create new connection: %v", err)
	}

	conn.SetControlMessage(ipv4.FlagTTL, true)
	defer conn.Close()

	icmpExamplePacket := icmp.CreateEchoPacket([]byte("heyy"))
	packetSize := icmpExamplePacket.Length() + ipv4.HeaderLen

	fmt.Printf("PING %v (%v) with %d(%d) bytes of data\n", flagAddressToConnect, destAddress, icmpExamplePacket.Data.Length(), packetSize)

	icmpDestAddr := net.UDPAddr{IP: destAddress}

	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT)
	go func() {
		<-interruptChan
		cancelCtx()
	}()

	receivedPacketStatsChan := make(chan receivedPacketInfo)
	sentPacketsCountChan := make(chan int)
	finalPacketsStatsChan := make(chan finalPacketsStats)

	go printFinalStatsAndExit(sentPacketsCountChan, finalPacketsStatsChan, destAddress)
	go printPacketStats(ctx, receivedPacketStatsChan, icmpExamplePacket.Length(), destAddress, flagAddressToConnect, finalPacketsStatsChan)
	go receiveEchoPacket(ctx, logger, conn, receivedPacketStatsChan, sentPacketsCountChan)

	if flagPacketsCount == 0 {
		packetCounter := 1
		for {
			go sendEchoPacket(logger, conn, &icmpDestAddr, packetCounter)

			packetCounter += 1
			time.Sleep(timeToSleepBetweenPackets)
		}
	} else {
		for i := 1; i <= flagPacketsCount; i += 1 {
			go sendEchoPacket(logger, conn, &icmpDestAddr, i)

			time.Sleep(timeToSleepBetweenPackets)
		}
		cancelCtx()
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

type receivedPacketInfo struct {
	SeqNumber int
	Time      time.Duration
	TTL       int
}

func sendEchoPacket(logger *zap.SugaredLogger, conn *ipv4.PacketConn, addr *net.UDPAddr, seqNumber int) {
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
}

func receiveEchoPacket(ctx context.Context, logger *zap.SugaredLogger, conn *ipv4.PacketConn, packetStatsChan chan<- receivedPacketInfo, sentPacketsCountChan chan<- int) {
	sentPacketsCount := 0

	for {
		buff := make([]byte, 1024)

		length, cm, _, err := conn.ReadFrom(buff)
		if err != nil {
			logger.Fatal(err)
		}

		select {
		case <-ctx.Done():
			fmt.Println("kak1")
			sentPacketsCountChan <- sentPacketsCount
			logger.Debug("goroutine with readpacket done")
			return
		default:
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

			packetStatsChan <- receivedPacketInfo{
				SeqNumber: int(reply.SequenceNumber),
				Time:      rttTime,
				TTL:       replyTTL,
			}

			sentPacketsCount += 1
		}
	}
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

type finalPacketsStats struct {
	minRTT               float64
	maxRTT               float64
	sumRTT               float64
	receivedPacketsCount int
}

func printPacketStats(ctx context.Context, packetStatsChan <-chan receivedPacketInfo, packetLength int, destAddressIP net.IP, destAddressDomain string, finalPacketsStatsChan chan<- finalPacketsStats) {
	receivedPacketsCount := 0
	minRTT := math.MaxFloat64
	maxRTT := 0.0
	sumRTT := 0.0

	for {
		select {
		case stats := <-packetStatsChan:

			rtt := float64(stats.Time.Microseconds()) / 1000

			if destAddressDomain == destAddressIP.String() {
				fmt.Printf("%d bytes from %v: icmp_seq=%d ttl=%d time=%.1f ms\n",
					packetLength, destAddressIP.String(), stats.SeqNumber, stats.TTL, rtt)
			} else {
				fmt.Printf("%d bytes from %v (%v): icmp_seq=%d ttl=%d time=%.1f ms\n",
					packetLength, destAddressDomain, destAddressIP.String(), stats.SeqNumber, stats.TTL, rtt)
			}

			if rtt > maxRTT {
				maxRTT = rtt
			}

			if rtt < minRTT {
				minRTT = rtt
			}

			sumRTT += rtt
			receivedPacketsCount += 1

		case <-ctx.Done():
			finalPacketsStatsChan <- finalPacketsStats{
				minRTT:               minRTT,
				maxRTT:               maxRTT,
				sumRTT:               sumRTT,
				receivedPacketsCount: receivedPacketsCount,
			}
			return
		}
	}
}

func printFinalStatsAndExit(sentPacketsCountChan <-chan int, finalPacketsStatsChan <-chan finalPacketsStats, destAddressIP net.IP) {
	sentPacketsCount := <-sentPacketsCountChan
	stats := <-finalPacketsStatsChan

	fmt.Printf("\n--- %v ping statistics ---\n", destAddressIP.String())
	fmt.Printf("%d packets transmitted, %d received, TODO packet loss, time TODO\n", sentPacketsCount, stats.receivedPacketsCount)
	fmt.Printf("rtt min/avg/max/mdev = %.3f/%.3f/%.3f/TODO ms\n", stats.minRTT, stats.sumRTT/float64(stats.receivedPacketsCount), stats.maxRTT)
	os.Exit(0)
}
