package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"strings"
)

func errOculus(message string) error {
	return fmt.Errorf("%s", message)
}

func oculusBytesToBits(data []byte) []byte {
	bits := make([]byte, 0, len(data)*8)
	for _, value := range data {
		for shift := 7; shift >= 0; shift-- {
			bits = append(bits, (value>>shift)&1)
		}
	}
	return bits
}

func oculusBitsToBytes(bits []byte) ([]byte, error) {
	if len(bits)%8 != 0 {
		return nil, errOculus("oculus bit length is not a multiple of 8")
	}

	out := make([]byte, len(bits)/8)
	for i := 0; i < len(bits); i += 8 {
		var value byte
		for _, bit := range bits[i : i+8] {
			value = (value << 1) | (bit & 1)
		}
		out[i/8] = value
	}
	return out, nil
}

func oculusNormalizeSeal(oracleSeal string) (string, []byte, error) {
	seal := strings.ToLower(strings.TrimSpace(oracleSeal))
	if len(seal) != 64 {
		return "", nil, errOculus("Oracle Seal must contain 64 hexadecimal characters")
	}

	raw, err := hex.DecodeString(seal)
	if err != nil || len(raw) != 32 {
		return "", nil, errOculus("Oracle Seal must be valid hexadecimal")
	}
	return seal, raw, nil
}

func encodeOculusPayload(oracleSeal string) ([]byte, error) {
	_, sealBytes, err := oculusNormalizeSeal(oracleSeal)
	if err != nil {
		return nil, err
	}

	message := make([]byte, 0, oculusRSK)
	message = append(message, byte(oculusVersion))
	message = append(message, sealBytes...)

	checksum := crc32.ChecksumIEEE(message)
	crcBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(crcBytes, checksum)
	message = append(message, crcBytes...)

	if len(message) != oculusRSK {
		return nil, errOculus("oculus message length mismatch")
	}

	nsym := oculusRSNSym()
	codeword, err := oculusRSEncode(message, nsym)
	if err != nil {
		return nil, err
	}
	if len(codeword) != oculusRSN() {
		return nil, errOculus("oculus codeword length mismatch")
	}
	if !oculusRSVerify(codeword, nsym) {
		return nil, errOculus("oculus Reed-Solomon self-check failed")
	}

	bits := oculusBytesToBits(codeword)
	if len(bits) != oculusPayloadBits() {
		return nil, errOculus("oculus petal bit capacity mismatch")
	}
	return bits, nil
}

func decodeOculusPayload(bits []byte) (string, error) {
	if len(bits) != oculusPayloadBits() {
		return "", errOculus("oculus petal bit capacity mismatch")
	}

	codeword, err := oculusBitsToBytes(bits)
	if err != nil {
		return "", err
	}
	if !oculusRSVerify(codeword, oculusRSNSym()) {
		return "", errOculus("oculus Reed-Solomon syndrome check failed")
	}

	message := codeword[:oculusRSK]
	version := message[0]
	sealBytes := message[1:33]
	storedCRC := binary.BigEndian.Uint32(message[33:37])

	if version != oculusVersion {
		return "", errOculus("oculus version mismatch")
	}

	checksum := crc32.ChecksumIEEE(message[:33])
	if checksum != storedCRC {
		return "", errOculus("oculus CRC mismatch")
	}

	return hex.EncodeToString(sealBytes), nil
}
