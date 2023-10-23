// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package merkledb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/maybe"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"io"
	"math"
)

const (
	boolLen              = 1
	trueByte             = 1
	falseByte            = 0
	minVarIntLen         = 1
	minMaybeByteSliceLen = boolLen
	minKeyLen            = minVarIntLen
	minByteSliceLen      = minVarIntLen
	minDBNodeLen         = minMaybeByteSliceLen + minVarIntLen
	minChildLen          = minVarIntLen + minKeyLen + ids.IDLen + boolLen
)

var (
	_ encoderDecoder = (*codecImpl)(nil)

	trueBytes  = []byte{trueByte}
	falseBytes = []byte{falseByte}

	errTooManyChildren    = errors.New("length of children list is larger than branching factor")
	errChildIndexTooLarge = errors.New("invalid child index. Must be less than branching factor")
	errLeadingZeroes      = errors.New("varint has leading zeroes")
	errInvalidBool        = errors.New("decoded bool is neither true nor false")
	errNonZeroKeyPadding  = errors.New("key partial byte should be padded with 0s")
	errExtraSpace         = errors.New("trailing buffer space")
	errIntOverflow        = errors.New("value overflows int")
)

// encoderDecoder defines the interface needed by merkleDB to marshal
// and unmarshal relevant types.
type encoderDecoder interface {
	encoder
	decoder
}

type encoder interface {
	dbNodeSize(n *dbNode) int
	// Assumes [n] is non-nil.
	encodeDBNode(n *dbNode) []byte
	// Assumes [hv] is non-nil.
	encodeHashValues(n *node) []byte
}

type decoder interface {
	// Assumes [n] is non-nil.
	decodeDBNode(tc TokenConfiguration, bytes []byte, n *dbNode) error
}

func newCodec() encoderDecoder {
	return &codecImpl{}
}

// Note that bytes.Buffer.Write always returns nil so we
// can ignore its return values in [codecImpl] methods.
type codecImpl struct{}

func (c *codecImpl) hashValuesSize(n *node) int {
	// total is storing the node's value + the factor number of children pointers + the child entries for n.childCount children
	total := maybeByteSliceSize(n.valueDigest) + uintSize(uint64(len(n.children))) + keySize(n.key)
	// for each non-nil entry, we add the additional size of the child entry
	for index, _ := range n.children {
		total += uintSize(uint64(index)) + len(ids.Empty)
	}
	return total
}

func (c *codecImpl) dbNodeSize(n *dbNode) int {
	// total is storing the node's value + the factor number of children pointers + the child entries for n.childCount children
	total := maybeByteSliceSize(n.value) + uintSize(uint64(len(n.children)))
	// for each non-nil entry, we add the additional size of the child entry
	for index, entry := range n.children {
		total += childSize(index, entry)
	}
	return total
}

func childSize(index byte, childEntry child) int {
	return uintSize(uint64(index)) + len(ids.Empty) + keySize(childEntry.compressedKey) + boolSize()
}

func maybeByteSliceSize(maybeValue maybe.Maybe[[]byte]) int {
	if maybeValue.HasValue() {
		return 1 + len(maybeValue.Value()) + uintSize(uint64(len(maybeValue.Value())))
	}
	return 1
}

func boolSize() int {
	return 1
}

var log128 = math.Log(128)

func uintSize(value uint64) int {
	if value == 0 {
		return 1
	}
	return 1 + int(math.Log(float64(value))/log128)
}

func keySize(p Key) int {
	return uintSize(uint64(p.bitLength)) + bytesNeeded(p.bitLength)
}

func (c *codecImpl) encodeDBNode(n *dbNode) []byte {
	startSize := c.dbNodeSize(n)
	buf := bytes.NewBuffer(make([]byte, 0, startSize))

	c.encodeMaybeByteSlice(buf, n.value)
	c.encodeUint(buf, uint64(len(n.children)))
	// Note we insert children in order of increasing index
	// for determinism.
	keys := maps.Keys(n.children)
	slices.Sort(keys)
	for _, index := range keys {
		entry := n.children[index]
		c.encodeUint(buf, uint64(index))
		c.encodeKey(buf, entry.compressedKey)
		c.encodeID(buf, entry.id)
		c.encodeBool(buf, entry.hasValue)
	}
	return buf.Bytes()
}

func (c *codecImpl) encodeID(dst io.Writer, id ids.ID) {
	for i := 0; i < len(ids.Empty); i++ {
		dst.Write([]byte{id[i]})
	}
}

func (c *codecImpl) encodeHashValues(n *node) []byte {
	buf := bytes.NewBuffer(make([]byte, 0, c.hashValuesSize(n)))
	c.encodeUint(buf, uint64(len(n.children)))

	// ensure that the order of entries is consistent
	keys := maps.Keys(n.children)
	slices.Sort(keys)
	for _, index := range keys {
		entry := n.children[index]
		c.encodeUint(buf, uint64(index))
		c.encodeID(buf, entry.id)
	}
	c.encodeMaybeByteSlice(buf, n.valueDigest)
	c.encodeKey(buf, n.key)
	return buf.Bytes()
}

func (c *codecImpl) decodeDBNode(tc TokenConfiguration, b []byte, n *dbNode) error {
	if minDBNodeLen > len(b) {
		return io.ErrUnexpectedEOF
	}

	src := &sliceReader{data: b}

	value, err := c.decodeMaybeByteSlice(src)
	if err != nil {
		return err
	}
	n.value = value

	numChildren, err := c.decodeUint(src)
	switch {
	case err != nil:
		return err
	case numChildren > uint64(tc.branchFactor):
		return errTooManyChildren
	case numChildren > uint64(src.Len()/minChildLen):
		return io.ErrUnexpectedEOF
	}
	n.children = make(map[byte]child, int(numChildren))
	var previousChild uint64
	for i := uint64(0); i < numChildren; i++ {
		index, err := c.decodeUint(src)
		if err != nil {
			return err
		}
		if index >= uint64(tc.branchFactor) || (i != 0 && index <= previousChild) {
			return errChildIndexTooLarge
		}
		previousChild = index

		compressedKey, err := c.decodeKey(src)
		if err != nil {
			return err
		}
		childID, err := c.decodeID(src)
		if err != nil {
			return err
		}
		hasValue, err := c.decodeBool(src)
		if err != nil {
			return err
		}
		n.children[byte(index)] = child{
			compressedKey: compressedKey,
			id:            childID,
			hasValue:      hasValue,
		}
	}
	if src.Len() != 0 {
		return errExtraSpace
	}
	return nil
}

func (*codecImpl) encodeBool(dst io.Writer, value bool) {
	bytesValue := falseBytes
	if value {
		bytesValue = trueBytes
	}
	_, _ = dst.Write(bytesValue)
}

func (*codecImpl) decodeBool(src *sliceReader) (bool, error) {
	boolByte, err := src.ReadByte()
	switch {
	case err == io.EOF:
		return false, io.ErrUnexpectedEOF
	case err != nil:
		return false, err
	case boolByte == trueByte:
		return true, nil
	case boolByte == falseByte:
		return false, nil
	default:
		return false, errInvalidBool
	}
}

func (*codecImpl) decodeUint(src *sliceReader) (uint64, error) {
	// To ensure encoding/decoding is canonical, we need to check for leading
	// zeroes in the varint.
	// The last byte of the varint we read is the most significant byte.
	// If it's 0, then it's a leading zero, which is considered invalid in the
	// canonical encoding.
	startLen := src.Len()
	val64, err := binary.ReadUvarint(src)
	if err != nil {
		if err == io.EOF {
			return 0, io.ErrUnexpectedEOF
		}
		return 0, err
	}
	endLen := src.Len()

	// Just 0x00 is a valid value so don't check if the varint is 1 byte
	if startLen-endLen > 1 {
		src.UnreadByte()
		lastByte, err := src.ReadByte()
		if err != nil {
			return 0, err
		}
		if lastByte == 0x00 {
			return 0, errLeadingZeroes
		}
	}

	return val64, nil
}

func (c *codecImpl) encodeUint(dst io.Writer, value uint64) error {
	i := 0
	for value >= 0x80 {
		if _, err := dst.Write([]byte{byte(value | 0x80)}); err != nil {
			return err
		}
		value >>= 7
		i++
	}
	_, err := dst.Write([]byte{byte(value)})
	return err
}

func (c *codecImpl) encodeMaybeByteSlice(dst io.Writer, maybeValue maybe.Maybe[[]byte]) {
	hasValue := maybeValue.HasValue()
	c.encodeBool(dst, hasValue)
	if hasValue {
		c.encodeByteSlice(dst, maybeValue.Value())
	}
}

func (c *codecImpl) decodeMaybeByteSlice(src *sliceReader) (maybe.Maybe[[]byte], error) {
	if minMaybeByteSliceLen > src.Len() {
		return maybe.Nothing[[]byte](), io.ErrUnexpectedEOF
	}

	if hasValue, err := c.decodeBool(src); err != nil || !hasValue {
		return maybe.Nothing[[]byte](), err
	}

	bytes, err := c.decodeByteSlice(src)
	if err != nil {
		return maybe.Nothing[[]byte](), err
	}

	return maybe.Some(bytes), nil
}

func (c *codecImpl) decodeByteSlice(src *sliceReader) ([]byte, error) {
	if minByteSliceLen > src.Len() {
		return nil, io.ErrUnexpectedEOF
	}

	length, err := c.decodeUint(src)
	switch {
	case err == io.EOF:
		return nil, io.ErrUnexpectedEOF
	case err != nil:
		return nil, err
	case length == 0:
		return nil, nil
	case length > uint64(src.Len()):
		return nil, io.ErrUnexpectedEOF
	}

	result, err := src.getSlice(int(length))
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return result, err
}

func (c *codecImpl) encodeByteSlice(dst io.Writer, value []byte) {
	c.encodeUint(dst, uint64(len(value)))
	if value != nil {
		_, _ = dst.Write(value)
	}
}

func (*codecImpl) decodeID(src *sliceReader) (ids.ID, error) {
	idBytes, err := src.getSlice(len(ids.Empty))
	if err != nil {
		return ids.Empty, err
	}
	return ids.ID(idBytes), nil
}

func (c *codecImpl) encodeKey(dst io.Writer, key Key) {
	c.encodeUint(dst, uint64(key.bitLength))
	_, _ = dst.Write(key.Bytes())
}

func (c *codecImpl) decodeKey(src *sliceReader) (Key, error) {
	if minKeyLen > src.Len() {
		return Key{}, io.ErrUnexpectedEOF
	}

	length, err := c.decodeUint(src)
	if err != nil {
		return Key{}, err
	}
	if length > math.MaxInt {
		return Key{}, errIntOverflow
	}
	result := Key{}
	result.bitLength = int(length)
	keyBytesLen := bytesNeeded(result.bitLength)
	if keyBytesLen > src.Len() {
		return Key{}, io.ErrUnexpectedEOF
	}
	buffer, err := src.getSlice(keyBytesLen)
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return Key{}, err
	}
	if result.hasPartialByte() {
		// Confirm that the padding bits in the partial byte are 0.
		// We want to only look at the bits to the right of the last token, which is at index length-1.
		// Generate a mask with (8-bitsToShift) 0s followed by bitsToShift 1s.
		paddingMask := byte(0xFF >> (8 - result.bitLength%8))
		if buffer[keyBytesLen-1]&paddingMask != 0 {
			return Key{}, errNonZeroKeyPadding
		}
	}
	result.value = string(buffer)
	return result, nil
}

type sliceReader struct {
	data   []byte
	offset int
}

func (sr *sliceReader) getSlice(length int) ([]byte, error) {
	if sr.Len() < length {
		return nil, io.ErrUnexpectedEOF
	}
	sr.offset += length
	return sr.data[sr.offset-length : sr.offset], nil
}

func (sr *sliceReader) UnreadByte() {
	sr.offset--
}

func (sr *sliceReader) ReadByte() (byte, error) {
	if sr.Len() < 1 {
		return 0, io.ErrUnexpectedEOF
	}
	val := sr.data[sr.offset]
	sr.offset++
	return val, nil
}

func (sr *sliceReader) Len() int {
	return len(sr.data) - sr.offset
}
