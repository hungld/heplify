package decoder

import (
	"bytes"
	"encoding/binary"
	"net"
	"os"

	"github.com/negbie/heplify/config"
	"github.com/negbie/heplify/ip4defrag"
	"github.com/negbie/heplify/logp"
	"github.com/tsg/gopacket"
	"github.com/tsg/gopacket/layers"
)

type Decoder struct {
	Host      string
	defragger *ip4defrag.IPv4Defragmenter
}

type Packet struct {
	Host      string
	Tsec      uint32
	Tmsec     uint32
	Srcip     uint32
	Dstip     uint32
	Sport     uint16
	Dport     uint16
	Payload   []byte
	SipHeader map[string][]string
}

func NewDecoder() *Decoder {
	host, err := os.Hostname()
	if err != nil {
		host = ""
	}
	return &Decoder{Host: host, defragger: ip4defrag.NewIPv4Defragmenter()}
}

func (d *Decoder) Process(data []byte, ci *gopacket.CaptureInfo) (*Packet, error) {
	pkt := &Packet{
		Host:  d.Host,
		Tsec:  uint32(ci.Timestamp.Unix()),
		Tmsec: uint32(ci.Timestamp.Nanosecond() / 1000),
	}

	packet := gopacket.NewPacket(data, layers.LayerTypeEthernet, gopacket.DecodeOptions{Lazy: true, NoCopy: true})
	if app := packet.ApplicationLayer(); app != nil {
		if config.Cfg.HepFilter != "" && bytes.Contains(app.Payload(), []byte(config.Cfg.HepFilter)) {
			return nil, nil
		}
	}

	for _, layer := range packet.Layers() {
		switch layer.LayerType() {

		case layers.LayerTypeIPv4:
			ip4l := packet.Layer(layers.LayerTypeIPv4)
			ip4, ok := ip4l.(*layers.IPv4)
			if !ok {
				break
			}
			if config.Cfg.Reasm {
				l := ip4.Length
				ip4, err := d.defragger.DefragIPv4(ip4)
				if err != nil {
					logp.Err("Error while defragging", err)
				} else if ip4 == nil {
					logp.Info("Recieved a fragment")
					continue
				}
				if ip4.Length != l {
					logp.Info("Decoding re-assembled packet: %s\n", ip4.NextLayerType())
					logp.Info(string(ip4.Payload))
					pb, ok := packet.(gopacket.PacketBuilder)
					if !ok {
						logp.Err("Error while getting packet builder: it's not a PacketBuilder")
					}
					nextDecoder := ip4.NextLayerType()
					nextDecoder.Decode(ip4.Payload, pb)
				}
			}
			pkt.Srcip = ip2int(ip4.SrcIP)
			pkt.Dstip = ip2int(ip4.DstIP)

		case layers.LayerTypeUDP:
			udpl := packet.Layer(layers.LayerTypeUDP)
			udp, ok := udpl.(*layers.UDP)
			if !ok {
				break
			}

			pkt.Sport = uint16(udp.SrcPort)
			pkt.Dport = uint16(udp.DstPort)
			pkt.Payload = udp.Payload

			p := gopacket.NewPacket(layer.LayerPayload(), LayerTypeSIP, gopacket.NoCopy)
			sipLayer, ok := p.Layers()[0].(*SIP)
			if !ok {
				break
			}
			pkt.SipHeader = sipLayer.Headers

			return pkt, nil

		case layers.LayerTypeTCP:
			tcpl := packet.Layer(layers.LayerTypeTCP)
			tcp, ok := tcpl.(*layers.TCP)
			if !ok {
				break
			}
			pkt.Sport = uint16(tcp.SrcPort)
			pkt.Dport = uint16(tcp.DstPort)
			pkt.Payload = tcp.Payload

			p := gopacket.NewPacket(layer.LayerPayload(), LayerTypeSIP, gopacket.NoCopy)
			sipLayer, ok := p.Layers()[0].(*SIP)
			if !ok {
				break
			}

			pkt.SipHeader = sipLayer.Headers

			return pkt, nil
		}
	}

	return nil, nil
}

func ip2int(ip net.IP) uint32 {
	if len(ip) == 16 {
		return binary.BigEndian.Uint32(ip[12:16])
	}
	return binary.BigEndian.Uint32(ip)
}
