package main

var (
	oculusGFExp [512]byte
	oculusGFLog [256]byte
)

func init() {
	x := 1
	for i := 0; i < 255; i++ {
		oculusGFExp[i] = byte(x)
		oculusGFLog[x] = byte(i)
		x <<= 1
		if x&0x100 != 0 {
			x ^= 0x11d
		}
	}
	for i := 255; i < 512; i++ {
		oculusGFExp[i] = oculusGFExp[i-255]
	}
}

func oculusGFMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return oculusGFExp[int(oculusGFLog[a])+int(oculusGFLog[b])]
}

func oculusGFPow(a byte, power int) byte {
	if power == 0 {
		return 1
	}
	if a == 0 {
		return 0
	}
	return oculusGFExp[(int(oculusGFLog[a])*power)%255]
}

func oculusPolyMul(p, q []byte) []byte {
	out := make([]byte, len(p)+len(q)-1)
	for i, a := range p {
		for j, b := range q {
			out[i+j] ^= oculusGFMul(a, b)
		}
	}
	return out
}

func oculusRSGenerator(nsym int) []byte {
	g := []byte{1}
	for i := 0; i < nsym; i++ {
		g = oculusPolyMul(g, []byte{1, oculusGFPow(2, i)})
	}
	return g
}

func oculusRSEncode(data []byte, nsym int) ([]byte, error) {
	if len(data)+nsym > 255 {
		return nil, errOculus("Reed-Solomon block too large")
	}

	gen := oculusRSGenerator(nsym)
	msg := make([]byte, len(data)+nsym)
	copy(msg, data)

	for i := 0; i < len(data); i++ {
		coef := msg[i]
		if coef == 0 {
			continue
		}
		for j := 0; j < len(gen); j++ {
			msg[i+j] ^= oculusGFMul(gen[j], coef)
		}
	}

	codeword := make([]byte, len(data)+nsym)
	copy(codeword, data)
	copy(codeword[len(data):], msg[len(data):])
	return codeword, nil
}

func oculusRSSyndromes(msg []byte, nsym int) []byte {
	synd := make([]byte, nsym)
	for i := 0; i < nsym; i++ {
		y := byte(0)
		alpha := oculusGFPow(2, i)
		for _, value := range msg {
			y = oculusGFMul(y, alpha) ^ value
		}
		synd[i] = y
	}
	return synd
}

func oculusRSVerify(codeword []byte, nsym int) bool {
	for _, s := range oculusRSSyndromes(codeword, nsym) {
		if s != 0 {
			return false
		}
	}
	return true
}
