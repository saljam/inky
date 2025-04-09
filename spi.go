package main

// from linux/spi/spidev.h
// https://www.kernel.org/doc/Documentation/spi/spidev

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	spi_cpha = 0x01
	spi_cpol = 0x02

	spi_mode_0 = (0 | 0)
	spi_mode_1 = (0 | spi_cpha)
	spi_mode_2 = (spi_cpol | 0)
	spi_mode_3 = (spi_cpol | spi_cpha)

	spi_ioc_wr_mode         = 0x40016b01 // _IOW(SPI_IOC_MAGIC, 1, __u8)
	spi_ioc_wr_max_speed_hz = 0x40046b04 // _IOW(SPI_IOC_MAGIC, 4, __u32)
)

type spi_ioc_transfer struct {
	tx_buf        uint64
	rx_buf        uint64
	len           uint32
	speed_hz      uint32
	delay_usecs   uint16
	bits_per_word uint8
	cs_change     uint8
	tx_nbits      uint8
	rx_nbits      uint8
	pad           uint16
}

func openSPI(dev string, mode uint8, speedHz uint32) (*os.File, error) {
	f, err := os.OpenFile(dev, os.O_RDWR, os.ModeDevice)
	if err != nil {
		return nil, err
	}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), spi_ioc_wr_mode, uintptr(unsafe.Pointer(&mode)))
	if errno != 0 {
		f.Close()
		return nil, fmt.Errorf("error setting mode to %v: %v", mode, syscall.Errno(errno))
	}
	_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), spi_ioc_wr_max_speed_hz, uintptr(unsafe.Pointer(&speedHz)))
	if errno != 0 {
		f.Close()
		return nil, fmt.Errorf("error setting speed to %v: %v", speedHz, syscall.Errno(errno))
	}
	return f, nil
}

func writeSPI(f *os.File, buf []byte) (n int, err error) {
	r := make([]byte, len(buf))
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(f.Fd()),
		uintptr(0x40006B00+(1*0x200000)),
		uintptr(unsafe.Pointer(&spi_ioc_transfer{
			tx_buf: uint64(uintptr(unsafe.Pointer(&buf[0]))),
			rx_buf: uint64(uintptr(unsafe.Pointer(&r[0]))),
			len:    uint32(len(buf)),
		})))
	if errno != 0 {
		return 0, fmt.Errorf("error writing to spi: %v", syscall.Errno(errno))
	}
	return len(buf), nil
}
