package main

import (
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"os/signal"
	"ping/internal/icmp"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/ipv4"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
)

var (
	flagAddressToConnect string
	logLevel             zapcore.Level
	flagPacketsCount     int
	flagIsPrivileged     bool
	flagDataToTransfer   []byte
	flagTimeToSleep      time.Duration
)

var (
	defaultAddressToBind             = net.ParseIP("0.0.0.0")
	defaultDataInPacket              = []byte("heyy")
	defaultTimeToSleepBetweenPackets = 1 * time.Second
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

	// init connection
	var conn *ipv4.PacketConn
	if flagIsPrivileged {
		conn, err = icmp.NewPrivilegedIPv4Connection(defaultAddressToBind.String())
	} else {
		conn, err = icmp.NewUnprivilegedIPv4Connection(defaultAddressToBind)
	}
	if err != nil {
		logger.Fatalf("cannot create new connection: %v", err)
	}

	// this option allows to receive ttl from control message
	conn.SetControlMessage(ipv4.FlagTTL, true)
	defer conn.Close()

	// create packet to examine its size
	icmpPacketForStats := icmp.CreateEchoPacket(flagDataToTransfer)
	packetSize := icmpPacketForStats.Length() + ipv4.HeaderLen

	fmt.Printf("PING %v (%v) with %d(%d) bytes of data\n", flagAddressToConnect, destAddress, icmpPacketForStats.Data.Length(), packetSize)

	// notify via channel when ctrl+C is pressed
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT)

	receivedPacketStatsChan := make(chan receivedPacketInfo)
	sentPacketsCountChan := make(chan int)

	// create waitgroup for waiting until statistics are printed
	var wg sync.WaitGroup
	wg.Add(1)

	go printPacketStats(&wg, receivedPacketStatsChan, icmpPacketForStats.Length(), destAddress, flagAddressToConnect, sentPacketsCountChan)
	go receiveEchoPacket(logger, conn, receivedPacketStatsChan)

	for i := 1; i != flagPacketsCount+1; i += 1 {
		select {
		case <-interruptChan:
			sentPacketsCountChan <- i
			wg.Wait()
			os.Exit(0)
		default:
		}
		go sendEchoPacket(logger, conn, &net.UDPAddr{IP: destAddress}, i)
		time.Sleep(flagTimeToSleep)
	}
	sentPacketsCountChan <- flagPacketsCount
	wg.Wait()
	os.Exit(0)
}

func init() {
	flag.IntVarP(&flagPacketsCount, "count", "c", -1, "stop after <count> replies")
	flag.BoolVar(&flagIsPrivileged, "privileged", false, "run with ip:icmp socket instead udp")
	flag.BytesBase64Var(&flagDataToTransfer, "data", defaultDataInPacket, "data to transfer in package body")
	flag.DurationVar(&flagTimeToSleep, "delay", defaultTimeToSleepBetweenPackets, "delay between sending packets")
	isDebug := flag.BoolP("debug", "d", false, "print debug messages")
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
	icmpEchoPacket := icmp.CreateEchoPacket(flagDataToTransfer)

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

func receiveEchoPacket(logger *zap.SugaredLogger, conn *ipv4.PacketConn, packetStatsChan chan<- receivedPacketInfo) {
	for {
		buff := make([]byte, 1024)

		length, cm, _, err := conn.ReadFrom(buff)
		if err != nil {
			logger.Fatalf("cannot read from connection: %v", err)
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

		packetStatsChan <- receivedPacketInfo{
			SeqNumber: int(reply.SequenceNumber),
			Time:      rttTime,
			TTL:       replyTTL,
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

func printPacketStats(wg *sync.WaitGroup, packetStatsChan <-chan receivedPacketInfo, packetLength int, destAddressIP net.IP, destAddressDomain string, sentPacketsCountChan <-chan int) {
	startTime := time.Now()

	receivedPacketsCount := 0
	minRTT := math.MaxFloat64
	maxRTT := 0.0

	sumRTT := 0.0
	sumRTT2 := 0.0

	for {
		select {
		case stats := <-packetStatsChan:

			rtt := float64(stats.Time.Microseconds()) / 1000

			if destAddressDomain == destAddressIP.String() {
				fmt.Printf("%d bytes from %v: icmp_seq=%d ttl=%d time=%.2f ms\n",
					packetLength, destAddressIP.String(), stats.SeqNumber, stats.TTL, rtt)
			} else {
				fmt.Printf("%d bytes from %v (%v): icmp_seq=%d ttl=%d time=%.2f ms\n",
					packetLength, destAddressDomain, destAddressIP.String(), stats.SeqNumber, stats.TTL, rtt)
			}

			if rtt > maxRTT {
				maxRTT = rtt
			}

			if rtt < minRTT {
				minRTT = rtt
			}

			sumRTT += rtt
			sumRTT2 += rtt * rtt
			receivedPacketsCount += 1

		case sentPacketsCount := <-sentPacketsCountChan:
			defer wg.Done()

			sumRTT /= float64(receivedPacketsCount)
			sumRTT2 /= float64(receivedPacketsCount)

			mdev := math.Sqrt(sumRTT2 - sumRTT*sumRTT)
			packetLossPercent := (1 - float64(receivedPacketsCount)/float64(sentPacketsCount)) * 100

			fmt.Printf("\n--- %v ping statistics ---\n", destAddressIP.String())
			fmt.Printf("%d packets transmitted, %d received, %.0f%% packet loss, %d ms\n", sentPacketsCount, receivedPacketsCount,
				packetLossPercent, time.Since(startTime).Milliseconds())
			if receivedPacketsCount != 0 {
				fmt.Printf("rtt min/avg/max/mdev = %.3f/%.3f/%.3f/%.3f ms\n", minRTT, sumRTT, maxRTT, mdev)
			}

			return
		}
	}
}
