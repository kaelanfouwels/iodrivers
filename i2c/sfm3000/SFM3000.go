package sfm3000

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/go-daq/crc8"
	"periph.io/x/periph/conn"
)

const crcPolynomial = 0x31
const flowOffset = 32000
const flowScaleFactorAirN2 = 140
const flowScaleFactorO2 = 142.8

//SFM3000 is the i2C driver for the SFM3000 Low Pressure Drop Digital Flow Meter
type SFM3000 struct {
	i2c      conn.Conn
	crcTable *crc8.Table
	readMode bool
	isAir    bool
	address  uint8
	label    string
}

//NewSFM3000 create a new SFM3000 driver
func NewSFM3000(i2c conn.Conn, address uint8, isAir bool, label string) (*SFM3000, error) {

	return &SFM3000{
		i2c:      i2c,
		readMode: false,
		crcTable: crc8.MakeTable(crcPolynomial),
		isAir:    isAir,
		address:  address,
		label:    label,
	}, nil
}

//Label ..
func (e *SFM3000) Label() string {
	return e.label
}

//SoftReset ..
func (e *SFM3000) SoftReset() error {
	e.readMode = false

	w := []byte{0x20, 0x00}
	r := make([]byte, 0)
	err := e.i2c.Tx(w, r)
	if err != nil {
		return fmt.Errorf("failed to write command: %w", err)
	}

	return nil
}

//GetSerial ..
func (e *SFM3000) GetSerial() ([4]byte, error) {
	e.readMode = false

	serial := [4]byte{}

	w := []byte{0x31, 0xAE}
	r := make([]byte, 4)

	err := e.i2c.Tx(w, r)
	if err != nil {
		return serial, fmt.Errorf("failed to write command: %w", err)
	}

	if len(r) != 4 {
		return serial, fmt.Errorf("response length unexpected (bytes), got %v, expected %v", len(r), 4)
	}

	copy(serial[:], r)

	return serial, nil
}

//GetValue Returns data, crc, timestamp, error
func (e *SFM3000) GetValue() (float64, uint8, time.Time, error) {

	value, crc, tstamp, err := e.getRaw()
	if err != nil {
		return 0, 0, tstamp, err
	}

	var scalefactor float64
	if e.isAir {
		scalefactor = flowScaleFactorAirN2
	} else {
		scalefactor = flowScaleFactorO2
	}

	flow := (float64(value) - flowOffset) / float64(scalefactor)
	return flow, crc, tstamp, nil
}

//getRaw Returns data uint16, crc uint8, error
func (e *SFM3000) getRaw() (uint16, uint8, time.Time, error) {

	if !(e.readMode) {

		w := []byte{0x10, 00}
		r := make([]byte, 0)

		err := e.i2c.Tx(w, r)
		if err != nil {
			return 0, 0, time.Time{}, fmt.Errorf("failed to write command: %w", err)
		}

		time.Sleep(200 * time.Millisecond) // Wait for sensor to change mode

		e.readMode = true
	}

	w := make([]byte, 0)
	r := make([]byte, 3)
	err := e.i2c.Tx(w, r)
	if err != nil {
		return 0, 0, time.Time{}, fmt.Errorf("failed to read command: %w", err)
	}
	if len(r) != 3 {
		return 0, 0, time.Time{}, fmt.Errorf("response length unexpected (bytes), got %v, expected %v", len(r), 3)
	}

	timestamp := time.Now()

	dataCRC := byte(crc8.Checksum(r[:2], e.crcTable))
	sensorCRC := r[2]

	if dataCRC != sensorCRC {
		return 0, 0, time.Time{}, fmt.Errorf("CRC Check failed, got %v, expected %v", sensorCRC, dataCRC)
	}

	data := binary.BigEndian.Uint16(r[:2])

	return data, dataCRC, timestamp, nil
}
