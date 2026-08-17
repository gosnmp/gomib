package model

import (
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
)

// IndexValueKind identifies the type of a decoded index value.
type IndexValueKind int

const (
	IndexValueIpAddress        IndexValueKind = iota + 1 // four-arc IPv4 address
	IndexValueOctetString                                // octet string (fixed, length-prefixed, or implied)
	IndexValueObjectIdentifier                           // sub-OID (length-prefixed or implied)
	IndexValueInteger32
	IndexValueUnsigned32
	IndexValueGauge32
	IndexValueTimeTicks
	IndexValueCounter32
	IndexValueBits
	IndexValueOpaque
)

// String returns a short name for the kind.
func (k IndexValueKind) String() string {
	switch k {
	case IndexValueIpAddress:
		return "ip-address"
	case IndexValueOctetString:
		return "octet-string"
	case IndexValueObjectIdentifier:
		return "object-identifier"
	case IndexValueInteger32:
		return "integer32"
	case IndexValueUnsigned32:
		return "unsigned32"
	case IndexValueGauge32:
		return "gauge32"
	case IndexValueTimeTicks:
		return "timeticks"
	case IndexValueCounter32:
		return "counter32"
	case IndexValueBits:
		return "bits"
	case IndexValueOpaque:
		return "opaque"
	default:
		return "unknown"
	}
}

// IndexValue is a typed value decoded from OID instance suffix arcs.
//
// The kind is determined by the compiled index schema and base type,
// following RFC 2578 section 7.7:
//
//   - Integer types retain their precise semantic application kind.
//   - IpAddress produces [IndexValueIpAddress] (four arcs).
//   - OCTET STRING, Opaque, and BITS retain their precise semantic kind and
//     are fixed-length, length-prefixed, or implied.
//   - OBJECT IDENTIFIER produces [IndexValueObjectIdentifier],
//     either length-prefixed or implied.
type IndexValue struct {
	kind  IndexValueKind
	value any // int32 | uint32 | [4]byte | []byte | OID
}

// Kind returns the type of the index value.
func (v IndexValue) Kind() IndexValueKind { return v.kind }

// NewInteger32IndexValue returns a signed Integer32 index value.
func NewInteger32IndexValue(value int32) IndexValue {
	return IndexValue{kind: IndexValueInteger32, value: value}
}

// NewUnsigned32IndexValue returns an Unsigned32 index value.
func NewUnsigned32IndexValue(value uint32) IndexValue {
	return IndexValue{kind: IndexValueUnsigned32, value: value}
}

// NewGauge32IndexValue returns a Gauge32 index value.
func NewGauge32IndexValue(value uint32) IndexValue {
	return IndexValue{kind: IndexValueGauge32, value: value}
}

// NewTimeTicksIndexValue returns a TimeTicks index value.
func NewTimeTicksIndexValue(value uint32) IndexValue {
	return IndexValue{kind: IndexValueTimeTicks, value: value}
}

// NewCounter32IndexValue returns a mechanically representable Counter32 index value.
func NewCounter32IndexValue(value uint32) IndexValue {
	return IndexValue{kind: IndexValueCounter32, value: value}
}

// NewIpAddressIndexValue returns an IPv4 index value.
func NewIpAddressIndexValue(value [4]byte) IndexValue {
	return IndexValue{kind: IndexValueIpAddress, value: value}
}

// NewOctetStringIndexValue returns an owned OCTET STRING index value.
func NewOctetStringIndexValue(value []byte) IndexValue {
	return IndexValue{kind: IndexValueOctetString, value: slices.Clone(value)}
}

// NewBitsIndexValue returns an owned BITS index value.
func NewBitsIndexValue(value []byte) IndexValue {
	return IndexValue{kind: IndexValueBits, value: slices.Clone(value)}
}

// NewOpaqueIndexValue returns an owned Opaque index value.
func NewOpaqueIndexValue(value []byte) IndexValue {
	return IndexValue{kind: IndexValueOpaque, value: slices.Clone(value)}
}

// NewObjectIdentifierIndexValue returns an owned OBJECT IDENTIFIER index value.
func NewObjectIdentifierIndexValue(value OID) IndexValue {
	return IndexValue{kind: IndexValueObjectIdentifier, value: slices.Clone(value)}
}

// Signed returns an Integer32 value.
func (v IndexValue) Signed() (int32, bool) {
	if v.kind == IndexValueInteger32 {
		return v.value.(int32), true
	}
	return 0, false
}

// Unsigned returns an unsigned integer-like index value.
func (v IndexValue) Unsigned() (uint32, bool) {
	switch v.kind {
	case IndexValueUnsigned32, IndexValueGauge32, IndexValueTimeTicks, IndexValueCounter32:
		return v.value.(uint32), true
	default:
		return 0, false
	}
}

// IpAddress returns the four-octet IPv4 address and true if this is an
// IP address index, or (zero, false) otherwise.
func (v IndexValue) IpAddress() ([4]byte, bool) {
	if v.kind == IndexValueIpAddress {
		return v.value.([4]byte), true
	}
	return [4]byte{}, false
}

// IP returns the IPv4 address as [net.IP], or nil if this is not an
// IP address index.
func (v IndexValue) IP() net.IP {
	if v.kind == IndexValueIpAddress {
		b := v.value.([4]byte)
		return net.IPv4(b[0], b[1], b[2], b[3])
	}
	return nil
}

// Bytes returns the octet string value and true, or (nil, false).
func (v IndexValue) Bytes() ([]byte, bool) {
	if v.kind == IndexValueOctetString || v.kind == IndexValueBits || v.kind == IndexValueOpaque {
		return slices.Clone(v.value.([]byte)), true
	}
	return nil, false
}

// OID returns the object identifier arcs and true, or (nil, false).
func (v IndexValue) OID() (OID, bool) {
	if v.kind == IndexValueObjectIdentifier {
		return slices.Clone(v.value.(OID)), true
	}
	return nil, false
}

// String returns a human-readable representation of the value.
func (v IndexValue) String() string {
	switch v.kind {
	case IndexValueInteger32:
		return strconv.FormatInt(int64(v.value.(int32)), 10)
	case IndexValueUnsigned32, IndexValueGauge32, IndexValueTimeTicks, IndexValueCounter32:
		return strconv.FormatUint(uint64(v.value.(uint32)), 10)
	case IndexValueIpAddress:
		b := v.value.([4]byte)
		return net.IPv4(b[0], b[1], b[2], b[3]).String()
	case IndexValueOctetString, IndexValueBits, IndexValueOpaque:
		data := v.value.([]byte)
		if isPrintableASCII(data) {
			return string(data)
		}
		var sb strings.Builder
		for i, b := range data {
			if i > 0 {
				sb.WriteByte(':')
			}
			fmt.Fprintf(&sb, "%02X", b)
		}
		return sb.String()
	case IndexValueObjectIdentifier:
		return v.value.(OID).String()
	default:
		return ""
	}
}

func isPrintableASCII(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	for _, b := range data {
		if b < 0x20 || b > 0x7E {
			return false
		}
	}
	return true
}

// arcsToBytes converts OID arcs to bytes, returning false if any arc exceeds 255.
func arcsToBytes(arcs []uint32) ([]byte, bool) {
	bytes := make([]byte, len(arcs))
	for i, a := range arcs {
		if a > 255 {
			return nil, false
		}
		bytes[i] = byte(a)
	}
	return bytes, true
}
