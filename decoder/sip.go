package decoder

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/tsg/gopacket"
	"github.com/tsg/gopacket/layers"
)

var LayerTypeSIP = gopacket.RegisterLayerType(2000, gopacket.LayerTypeMetadata{Name: "SIP", Decoder: gopacket.DecodeFunc(decodeSIP)})

// SIPVersion defines the different versions of the SIP Protocol
type SIPVersion uint8

// Represents all the versions of SIP protocol
const (
	SIPVersion1 SIPVersion = 1
	SIPVersion2 SIPVersion = 2
)

func (sv SIPVersion) String() string {
	switch sv {
	default:
		// Defaulting to SIP/2.0
		return "SIP/2.0"
	case SIPVersion1:
		return "SIP/1.0"
	case SIPVersion2:
		return "SIP/2.0"
	}
}

// GetSIPVersion is used to get SIP version constant
func GetSIPVersion(version string) (SIPVersion, error) {
	switch strings.ToUpper(version) {
	case "SIP/1.0":
		return SIPVersion1, nil
	case "SIP/2.0":
		return SIPVersion2, nil
	default:
		return 0, fmt.Errorf("invalid SIP version: '%s'", version)

	}
}

// SIPMethod defines the different methods of the SIP Protocol
// defined in the different RFC's
type SIPMethod uint16

// Here are all the SIP methods
const (
	SIPMethodInvite    SIPMethod = 1  // INVITE	[RFC3261]
	SIPMethodAck       SIPMethod = 2  // ACK	[RFC3261]
	SIPMethodBye       SIPMethod = 3  // BYE	[RFC3261]
	SIPMethodCancel    SIPMethod = 4  // CANCEL	[RFC3261]
	SIPMethodOptions   SIPMethod = 5  // OPTIONS	[RFC3261]
	SIPMethodRegister  SIPMethod = 6  // REGISTER	[RFC3261]
	SIPMethodPrack     SIPMethod = 7  // PRACK	[RFC3262]
	SIPMethodSubscribe SIPMethod = 8  // SUBSCRIBE	[RFC6665]
	SIPMethodNotify    SIPMethod = 9  // NOTIFY	[RFC6665]
	SIPMethodPublish   SIPMethod = 10 // PUBLISH	[RFC3903]
	SIPMethodInfo      SIPMethod = 11 // INFO	[RFC6086]
	SIPMethodRefer     SIPMethod = 12 // REFER	[RFC3515]
	SIPMethodMessage   SIPMethod = 13 // MESSAGE	[RFC3428]
	SIPMethodUpdate    SIPMethod = 14 // UPDATE	[RFC3311]
	SIPMethodPing      SIPMethod = 15 // PING	[https://tools.ietf.org/html/draft-fwmiller-ping-03]
)

func (sm SIPMethod) String() string {
	switch sm {
	default:
		return "Unknown method"
	case SIPMethodInvite:
		return "INVITE"
	case SIPMethodAck:
		return "ACK"
	case SIPMethodBye:
		return "BYE"
	case SIPMethodCancel:
		return "CANCEL"
	case SIPMethodOptions:
		return "OPTIONS"
	case SIPMethodRegister:
		return "REGISTER"
	case SIPMethodPrack:
		return "PRACK"
	case SIPMethodSubscribe:
		return "SUBSCRIBE"
	case SIPMethodNotify:
		return "NOTIFY"
	case SIPMethodPublish:
		return "PUBLISH"
	case SIPMethodInfo:
		return "INFO"
	case SIPMethodRefer:
		return "REFER"
	case SIPMethodMessage:
		return "MESSAGE"
	case SIPMethodUpdate:
		return "UPDATE"
	case SIPMethodPing:
		return "PING"
	}
}

// GetSIPMethod returns the constant of a SIP method
// from its string
func GetSIPMethod(method string) (SIPMethod, error) {
	switch strings.ToUpper(method) {
	case "INVITE":
		return SIPMethodInvite, nil
	case "ACK":
		return SIPMethodAck, nil
	case "BYE":
		return SIPMethodBye, nil
	case "CANCEL":
		return SIPMethodCancel, nil
	case "OPTIONS":
		return SIPMethodOptions, nil
	case "REGISTER":
		return SIPMethodRegister, nil
	case "PRACK":
		return SIPMethodPrack, nil
	case "SUBSCRIBE":
		return SIPMethodSubscribe, nil
	case "NOTIFY":
		return SIPMethodNotify, nil
	case "PUBLISH":
		return SIPMethodPublish, nil
	case "INFO":
		return SIPMethodInfo, nil
	case "REFER":
		return SIPMethodRefer, nil
	case "MESSAGE":
		return SIPMethodMessage, nil
	case "UPDATE":
		return SIPMethodUpdate, nil
	case "PING":
		return SIPMethodPing, nil
	default:
		return 0, fmt.Errorf("invalid SIP method: '%s'", method)
	}
}

// SIP object will contains information about decoded SIP packet.
// -> The SIP Version
// -> The SIP Headers (in a map[string][]string because of multiple headers with the same name
// -> The SIP Method (if it's a request)
// -> The SIP Response code (if it's a response)
// -> The SIP Status line (if it's a response)
// You can easily know the type of the packet with the IsResponse boolean
//
type SIP struct {
	layers.BaseLayer

	// Base information
	Version SIPVersion
	Headers map[string][]string

	// Request
	Method SIPMethod

	// Response
	IsResponse     bool
	ResponseCode   int
	ResponseStatus string
}

// decodeSIP decodes the byte slice into a SIP type. It also
// setups the application Layer in PacketBuilder.
func decodeSIP(data []byte, p gopacket.PacketBuilder) error {
	s := NewSIP()
	err := s.DecodeFromBytes(data, p)
	if err != nil {
		return err
	}
	p.AddLayer(s)
	p.SetApplicationLayer(s)
	return nil
}

// NewSIP instantiates a new empty SIP object
func NewSIP() *SIP {
	s := new(SIP)
	s.Headers = make(map[string][]string)
	return s
}

func (s *SIP) CanDecode() gopacket.LayerClass {
	return LayerTypeSIP
}

func (s *SIP) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

// LayerType returns gopacket.LayerTypeSIP.
func (s *SIP) LayerType() gopacket.LayerType {
	return LayerTypeSIP
}

// Payload returns the base layer payload
func (s *SIP) Payload() []byte {
	return s.BaseLayer.Payload
}

// DecodeFromBytes decodes the slice into the SIP struct.
func (s *SIP) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {

	// Init some vars for parsing follow-up
	var countLines int
	var line []byte
	var err error

	// Clean leading new line
	data = bytes.Trim(data, "\n")

	// Iterate on all lines of the SIP Headers
	// and stop when we reach the SDP (aka when the new line
	// is at index 0 of the remaining packet)
	buffer := bytes.NewBuffer(data)

	for {

		// Read next line
		line, err = buffer.ReadBytes(byte('\n'))
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return err
			}
		}

		// Trim the new line delimiters
		line = bytes.Trim(line, "\r\n")

		// Empty line, we hit Body
		// Putting packet remain in Paypload
		if len(line) == 0 {
			s.BaseLayer.Payload = buffer.Bytes()
			break
		}

		// First line is the SIP request/response line
		// Other lines are headers
		if countLines == 0 {
			err = s.ParseFirstLine(line)
			if err != nil {
				return err
			}

		} else {

			// Find the ':' to separate header name and value
			index := bytes.Index(line, []byte(":"))
			if index >= 0 {

				headerName := strings.ToLower(string(bytes.Trim(line[:index], " ")))
				headerValue := string(bytes.Trim(line[index+1:], " "))

				s.Headers[headerName] = append(s.Headers[headerName], headerValue)
			}
		}

		countLines++
	}

	return nil
}

// ParseFirstLine will compute the first line of a SIP packet.
// The first line will tell us if it's a request or a response.
//
// Examples of first line of SIP Prococol :
//
// 	Request 	: INVITE bob@example.com SIP/2.0
// 	Response 	: SIP/2.0 200 OK
// 	Response	: SIP/2.0 501 Not Implemented
//
func (s *SIP) ParseFirstLine(firstLine []byte) error {

	var err error

	// Splits line by space
	splits := strings.SplitN(string(firstLine), " ", 3)

	// We must have at least 3 parts
	if len(splits) < 3 {
		return fmt.Errorf("invalid first SIP line: '%s'", string(firstLine))
	}

	// Determine the SIP packet type
	if strings.HasPrefix(splits[0], "SIP") {

		// --> Response
		s.IsResponse = true

		// Validate SIP Version
		s.Version, err = GetSIPVersion(splits[0])
		if err != nil {
			return err
		}

		// Compute code
		s.ResponseCode, err = strconv.Atoi(splits[1])
		if err != nil {
			return err
		}

		// Compute status line
		s.ResponseStatus = splits[2]

	} else {

		// --> Request

		// Validate method
		s.Method, err = GetSIPMethod(splits[0])
		if err != nil {
			return err
		}

		// Validate SIP Version
		s.Version, err = GetSIPVersion(splits[2])
		if err != nil {
			return err
		}
	}

	return nil
}

// GetAllHeaders will return the full headers of the
// current SIP packets in a map[string][]string
func (s *SIP) GetAllHeaders() map[string][]string {
	return s.Headers
}

// GetHeader will return all the headers with
// the specified name.
func (s *SIP) GetHeader(headerName string) []string {
	headerName = strings.ToLower(headerName)
	h := make([]string, 0)
	if _, ok := s.Headers[headerName]; ok {
		if len(s.Headers[headerName]) > 0 {
			return s.Headers[headerName]
		}
	}
	return h
}

// GetFirstHeader will return the first header with
// the specified name. If the current SIP packet has multiple
// headers with the same name, it returns the first.
func (s *SIP) GetFirstHeader(headerName string) string {
	headerName = strings.ToLower(headerName)
	if _, ok := s.Headers[headerName]; ok {
		if len(s.Headers[headerName]) > 0 {
			return s.Headers[headerName][0]
		}
	}
	return ""
}
