package buffer

// BitReader reads MSB-first bits from a byte slice.
type BitReader struct {
	data []byte
	bytePos int
	bitPos uint8
}

func NewBitReader(data []byte) *BitReader {
	return &BitReader{data: data}
}

func (r *BitReader) BytesLeft() int {
	if r.bytePos >= len(r.data) {
		return 0
	}
	return len(r.data) - r.bytePos
}

func (r *BitReader) ReadBit() (uint64, bool) {
	if r.bytePos >= len(r.data) {
		return 0, false
	}
	b := r.data[r.bytePos]
	bit := (b >> (7 - r.bitPos)) & 0x01
	r.bitPos++
	if r.bitPos == 8 {
		r.bitPos = 0
		r.bytePos++
	}
	return uint64(bit), true
}

func (r *BitReader) ReadBits(n int) (uint64, bool) {
	if n <= 0 {
		return 0, true
	}
	var v uint64
	for i := 0; i < n; i++ {
		bit, ok := r.ReadBit()
		if !ok {
			return 0, false
		}
		v = (v << 1) | bit
	}
	return v, true
}

func (r *BitReader) ReadByte() (byte, bool) {
	if r.bitPos == 0 {
		if r.bytePos >= len(r.data) {
			return 0, false
		}
		b := r.data[r.bytePos]
		r.bytePos++
		return b, true
	}
	val, ok := r.ReadBits(8)
	if !ok {
		return 0, false
	}
	return byte(val), true
}

func (r *BitReader) SkipBits(n int) bool {
	_, ok := r.ReadBits(n)
	return ok
}

// ReadUE reads an unsigned Exp-Golomb code.
func (r *BitReader) ReadUE() (uint64, bool) {
	zeros := 0
	for {
		bit, ok := r.ReadBit()
		if !ok {
			return 0, false
		}
		if bit == 0 {
			zeros++
			continue
		}
		break
	}
	if zeros == 0 {
		return 0, true
	}
	value, ok := r.ReadBits(zeros)
	if !ok {
		return 0, false
	}
	return (1<<zeros - 1) + value, true
}

// ReadSE reads a signed Exp-Golomb code.
func (r *BitReader) ReadSE() (int64, bool) {
	ue, ok := r.ReadUE()
	if !ok {
		return 0, false
	}
	k := int64(ue)
	if k%2 == 0 {
		return -(k / 2), true
	}
	return (k + 1) / 2, true
}
