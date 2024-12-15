package icmp

import (
	"bytes"
	"encoding/binary"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/ipv4"
)

type EchoPacket struct {
	Type           uint8
	Code           uint8
	Checksum       uint16
	Data           EchoPacketData
	Identifier     uint16
	SequenceNumber uint16
}

type EchoPacketData struct {
	Timestamp uint64
	RawData   []byte
}

func CreateEchoPacket(data []byte) *EchoPacket {
	return &EchoPacket{
		Type:     8,
		Code:     0,
		Checksum: 0,
		Data: EchoPacketData{
			Timestamp: uint64(time.Now().UnixMilli()),
			RawData:   data,
		},
		Identifier:     0,
		SequenceNumber: 0,
	}
}

func GetChecksum(raw []byte) uint16 {
	checksum := uint32(0)

	// if length of packet is odd, workaround by switch last byte to last octet (0x05 to 0x00 0x05)
	if len(raw)%2 != 0 {
		secondLastbyte := raw[len(raw)-2]
		raw[len(raw)-2] = 0x00
		raw = append(raw, secondLastbyte)
	}

	for i := 0; i < len(raw); i += 2 {
		octet := uint16(raw[i])<<8 + uint16(raw[i+1])
		checksum += uint32(octet)
	}

	checksum = checksum>>16 + checksum&0xffff

	return ^uint16(checksum)
}

func (p *EchoPacket) Prepare() ([]byte, error) {
	rawPacket, err := p.Marshal()
	if err != nil {
		return nil, errors.Wrap(err, "failed marshal packet")
	}

	checksum := GetChecksum(rawPacket)
	p.Checksum = checksum

	rawPacketWithChecksum, err := p.Marshal()
	if err != nil {
		return nil, errors.Wrap(err, "failed marshal packet with checksum")
	}

	return rawPacketWithChecksum, nil
}

func (p *EchoPacket) Marshal() ([]byte, error) {
	var err error
	buf := bytes.NewBuffer([]byte{})

	// 1 byte for type
	err = binary.Write(buf, binary.LittleEndian, p.Type)
	if err != nil {
		return nil, err
	}

	// 1 byte for code
	err = binary.Write(buf, binary.LittleEndian, p.Code)
	if err != nil {
		return nil, err
	}

	// 2 bytes for checksum
	err = binary.Write(buf, binary.BigEndian, p.Checksum)
	if err != nil {
		return nil, err
	}

	// 2 bytes for identifier
	err = binary.Write(buf, binary.BigEndian, p.Identifier)
	if err != nil {
		return nil, err
	}

	// 2 bytes for sequence number
	err = binary.Write(buf, binary.BigEndian, p.SequenceNumber)
	if err != nil {
		return nil, err
	}

	// 8 bytes for unix timestamp
	err = binary.Write(buf, binary.LittleEndian, p.Data.Timestamp)
	if err != nil {
		return nil, err
	}

	if p.Data.RawData != nil {
		err = binary.Write(buf, binary.BigEndian, p.Data.RawData)
		if err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil

}

func ParseEchoReplyPacket(rawPacket []byte) *EchoPacket {
	packet := new(EchoPacket)

	packet.Type = uint8(rawPacket[0])
	packet.Code = uint8(rawPacket[1])

	packet.Checksum = binary.BigEndian.Uint16(rawPacket[2:4])

	packet.Identifier = binary.BigEndian.Uint16(rawPacket[4:6])
	packet.SequenceNumber = binary.BigEndian.Uint16(rawPacket[6:8])

	packet.Data.Timestamp = binary.LittleEndian.Uint64(rawPacket[8:16])
	packet.Data.RawData = rawPacket[16:]

	return packet
}

func NewUnprivilegedIPv4Connection(ipAddr [4]byte) (*ipv4.PacketConn, error) {
	s, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_ICMP)
	if err != nil {
		return nil, err
	}

	sa := &syscall.SockaddrInet4{Addr: ipAddr}

	if err := syscall.Bind(s, sa); err != nil {
		syscall.Close(s)
		return nil, os.NewSyscallError("bind", err)
	}

	f := os.NewFile(uintptr(s), "datagram-oriented icmp")
	conn, err := net.FilePacketConn(f)
	if err != nil {
		return nil, err
	}

	f.Close()

	ipv4Conn := ipv4.NewPacketConn(conn)

	return ipv4Conn, nil
}

func NewPrivilegedIPv4Connection(ipAddr string) (*ipv4.PacketConn, error) {
	conn, err := net.ListenPacket("ip4:icmp", ipAddr)
	if err != nil {
		return nil, err
	}

	ipv4Conn := ipv4.NewPacketConn(conn)

	return ipv4Conn, nil
}
