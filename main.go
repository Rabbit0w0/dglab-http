package main

// 讲真的 开发这个项目 调试的时候是十分甚至九分痛苦

import (
	"bytes"
	"fmt"
	"github.com/bearmini/bitstream-go"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"strings"
	"tinygo.org/x/bluetooth"
)

// Types

type SetPowerRequest struct {
	PowerA uint `json:"powerA" binding:"required,min=0,max=2047"`
	PowerB uint `json:"powerB" binding:"required,min=0,max=2047"`
}

type SendWaveRequest struct {
	Channel uint `json:"channel" binding:"required,min=1,max=2"`
	ParamX  uint `json:"paramX" binding:"required,max=31"`
	ParamY  uint `json:"paramY" binding:"required,max=1023"`
	ParamZ  uint `json:"paramZ" binding:"required,max=31"`
}

// Functions

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}

func clamp(a, min, max int) int {
	if a <= min {
		return min
	}
	if a >= max {
		return max
	}
	return a
}

func dumpBoolArr(arr []bool) string {
	r := ""
	for i := 0; i < len(arr); i++ {
		if arr[i] {
			r += "1"
		} else {
			r += "0"
		}
	}
	return r
}

func reverseBool(input []bool) []bool {
	if len(input) == 0 {
		return input
	}
	return append(reverseBool(input[1:]), input[0])
}

func reverseByteArrayBits(input []byte) []byte {
	br := bytes.NewReader(input)
	r := bitstream.NewReader(br, nil)
	boolList := make([]bool, len(input)*8)
	for i := 0; i < len(input)*8; i++ {
		boolList[i], _ = r.ReadBool()
	}
	boolList = reverseBool(boolList)
	b := bytes.NewBuffer([]byte{})
	nw := bitstream.NewWriter(b)
	for i := 0; i < len(input)*8; i++ {
		nw.WriteBool(boolList[i])
	}
	nw.Flush()
	return b.Bytes()
}

func NewDG16BitUUID(shortUUID uint32) bluetooth.UUID {
	// https://stackoverflow.com/questions/36212020/how-can-i-convert-a-bluetooth-16-bit-service-uuid-into-a-128-bit-uuid
	// In official doc:
	// 基础UUID: 955Axxxx-0FE2-F5AA-A094-84B8D4F3E8AD (将xxxx替换为服务的UUID)
	if shortUUID > 0xFFFF {
		panic("invalid short uuid")
	}
	shortUUID += 0x955A0000 // Add value to the first part
	var uuid bluetooth.UUID
	uuid[0] = 0xD4F3E8AD
	uuid[1] = 0xA09484B8
	uuid[2] = 0x0FE2F5AA
	uuid[3] = shortUUID
	return uuid
}

func ConvertToBits(b uint, end int) []bool {
	result := make([]bool, 12)
	s := fmt.Sprintf("%b", b)
	if len(s) < end+1 {
		t := strings.Repeat("0", end+1-len(s))
		s = t + s
	}
	for i := 0; i <= 11; i++ {
		if i >= len(s) || i < 0 {
			continue
		}
		result[i] = s[i] == '1'
	}
	// dumpBoolArr(result)
	return result
}

// Constants

var BatteryLevelSrv = NewDG16BitUUID(0x180A)
var BatteryLevelChr = NewDG16BitUUID(0x1500)

var PwmSrv = NewDG16BitUUID(0x180B)

var PwmAb2Chr = NewDG16BitUUID(0x1504)
var PwmA34Chr = NewDG16BitUUID(0x1505)
var PwmB34Chr = NewDG16BitUUID(0x1506)

var adapter = bluetooth.DefaultAdapter

// Realtime data
var batteryLevel = 0
var chanAPower uint8 = 0
var chanBPower uint8 = 0

func connectAddress() string {
	if len(os.Args) < 2 {
		println("usage: dglab-http [address]")
		os.Exit(1)
	}

	// look for device with specific name
	address := os.Args[1]

	return address
}

func main() {
	// Enable BLE interface.
	must("enable BLE stack", adapter.Enable())

	ch := make(chan bluetooth.ScanResult, 1)

	// Start scanning.
	println("Initializing bluetooth...")
	err := adapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
		// println("Found device: ", device.Address.String(), device.RSSI, device.LocalName())
		if device.Address.String() == connectAddress() {
			println("Found device! Connecting to " + device.LocalName())
			adapter.StopScan()
			ch <- device
		}
	})
	must("start scan", err)

	var device *bluetooth.Device
	select {
	case result := <-ch:
		device, err = adapter.Connect(result.Address, bluetooth.ConnectionParams{})
		if err != nil {
			println(err.Error())
			return
		}

		println("Connected to ", result.Address.String(), " (", result.LocalName(), ")")
	}

	// Disconnect before exit
	defer func() {
		err = device.Disconnect()
		if err != nil {
			println(err)
		}
	}()

	// get services
	println("Discovering services/characteristics")

	batterySrv, err := device.DiscoverServices([]bluetooth.UUID{BatteryLevelSrv})
	must("discover battery service", err)
	if len(batterySrv) == 0 {
		panic("could not find battery service")
	}
	batteryChr, err := batterySrv[0].DiscoverCharacteristics([]bluetooth.UUID{BatteryLevelChr})
	must("discover battery characteristic", err)
	if len(batteryChr) == 0 {
		panic("could not find battery characteristic")
	}

	must("enabling battery notification", batteryChr[0].EnableNotifications(func(buf []byte) {
		fmt.Printf("Battery level: %d\n", buf[0])
		batteryLevel = int(buf[0])
	}))

	// Update current battery level
	btr := make([]byte, 1)
	batteryChr[0].Read(btr)
	batteryLevel = int(btr[0])

	pwmSrv, err := device.DiscoverServices([]bluetooth.UUID{PwmSrv})
	must("discover pwm service", err)
	if len(pwmSrv) == 0 {
		panic("could not find pwm service")
	}
	ab2Chr, err := pwmSrv[0].DiscoverCharacteristics([]bluetooth.UUID{PwmAb2Chr})
	must("discover ab2 characteristic", err)
	if len(ab2Chr) == 0 {
		panic("could not find ab2 characteristic")
	}

	must("enabling ab2 notification", ab2Chr[0].EnableNotifications(func(buf []byte) {
		if len(buf) == 0 {
			return
		}
		buf = reverseByteArrayBits(buf)
		byteReader := bytes.NewReader(buf)
		r := bitstream.NewReader(byteReader, nil)
		chanAPower, _ = r.ReadNBitsAsUint8(11)
		chanBPower, _ = r.ReadNBitsAsUint8(11)
		// Drop the last 2 bits
	}))

	a34Chr, err := pwmSrv[0].DiscoverCharacteristics([]bluetooth.UUID{PwmA34Chr})
	must("discover a34 characteristic", err)
	if len(a34Chr) == 0 {
		panic("could not find a34 characteristic")
	}
	b34Chr, err := pwmSrv[0].DiscoverCharacteristics([]bluetooth.UUID{PwmB34Chr})
	must("discover b34 characteristic", err)
	if len(b34Chr) == 0 {
		panic("could not find b34 characteristic")
	}

	println("Bluetooth initialized.")

	println("Setting up Http service...")

	r := gin.Default()
	r.Any("/status", func(c *gin.Context) {
		btr := make([]byte, 1)
		batteryChr[0].Read(btr)
		batteryLevel = int(btr[0])

		btr = make([]byte, 3)
		ab2Chr[0].Read(btr)
		byteReader := bytes.NewReader(btr)
		r := bitstream.NewReader(byteReader, nil)
		// Dispose first 2 bits
		r.ReadBool()
		r.ReadBool()
		chanAPower, _ = r.ReadNBitsAsUint8(11)
		chanBPower, _ = r.ReadNBitsAsUint8(11)

		c.JSON(200, gin.H{
			"battery":    batteryLevel,
			"chanAPower": chanAPower,
			"chanBPower": chanBPower,
		})
	})
	r.POST("/setPower", func(c *gin.Context) {
		req := SetPowerRequest{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"message": fmt.Sprintf("malformed request: %s", err.Error()),
			})
			return
		}
		b := bytes.NewBuffer([]byte{})
		wr := bitstream.NewWriter(b)
		wr.WriteBool(false)
		wr.WriteBool(false)
		wr.WriteNBitsOfUint16BE(11, uint16(req.PowerA))
		wr.WriteNBitsOfUint16BE(11, uint16(req.PowerB))
		wr.Flush()
		_, err := ab2Chr[0].WriteWithoutResponse(b.Bytes()[:2])
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message":          "bluetooth exception",
				"bluetoothPayload": fmt.Sprintf("%b", b.Bytes()),
				"err":              err.Error(),
			})
			println(err.Error())
			return
		}
		chanAPower = uint8(req.PowerA)
		chanBPower = uint8(req.PowerB)
		c.JSON(http.StatusOK, gin.H{
			"message":          "ok",
			"bluetoothPayload": fmt.Sprintf("%b", b.Bytes()),
		})
	})
	r.POST("/sendWave", func(c *gin.Context) {
		req := SendWaveRequest{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"message": fmt.Sprintf("malformed request: %s", err.Error()),
			})
			return
		}
		b := bytes.NewBuffer([]byte{})
		wr := bitstream.NewWriter(b)
		// 20-23 bit
		wr.WriteBool(false)
		wr.WriteBool(false)
		wr.WriteBool(false)
		wr.WriteBool(false)
		wr.WriteNBitsOfUint8(5, uint8(req.ParamZ))
		wr.WriteNBitsOfUint16BE(10, uint16(req.ParamY))
		wr.WriteNBitsOfUint8(5, uint8(req.ParamX))
		wr.Flush()
		err = nil
		if req.Channel == 1 {
			_, err = a34Chr[0].WriteWithoutResponse(b.Bytes()[:2])
		} else {
			_, err = b34Chr[0].WriteWithoutResponse(b.Bytes()[:2])
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message":          "bluetooth exception",
				"bluetoothPayload": fmt.Sprintf("%b", b.Bytes()),
				"err":              err.Error(),
			})
			println(err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message":          "ok",
			"bluetoothPayload": fmt.Sprintf("%b", b.Bytes()),
		})
	})
	_ = r.Run(":8080")
}
