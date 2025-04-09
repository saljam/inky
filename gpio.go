package main

// from linux/gpio.h
// https://docs.kernel.org/userspace-api/gpio/chardev.html

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	gpio_max_name_size         = 32
	gpio_v2_line_num_attrs_max = 10
	gpio_v2_lines_max          = 64

	gpio_v2_line_flag_input          = 1 << 2
	gpio_v2_line_flag_output         = 1 << 3
	gpio_v2_line_flag_bias_pull_up   = 1 << 8
	gpio_v2_line_flag_bias_pull_down = 1 << 9
	gpio_v2_line_flag_bias_disabled  = 1 << 10

	gpio_v2_get_line_ioctl        = 0xc250b407 // _IOWR(0xB4, 0x07, struct gpio_v2_line_request)
	gpio_v2_line_set_values_ioctl = 0xc010b40f // _IOWR(0xB4, 0x0F, struct gpio_v2_line_values)
)

type gpio_v2_line_attribute struct {
	id      uint32
	padding uint32
	values  uint64
}

type gpio_v2_line_config_attribute struct {
	attr gpio_v2_line_attribute
	mask uint64
}

type gpio_v2_line_config struct {
	flags     uint64
	num_attrs uint32
	padding   [5]uint32
	attrs     [gpio_v2_line_num_attrs_max]gpio_v2_line_config_attribute
}

type gpio_v2_line_request struct {
	offsets           [gpio_v2_lines_max]uint32
	consumer          [gpio_max_name_size]byte
	config            gpio_v2_line_config
	num_lines         uint32
	event_buffer_size uint32
	padding           [5]uint32
	fd                int32
}

type gpio_v2_line_values struct {
	bits uint64
	mask uint64
}

func openGPIO(dev string, line uint32, flags uint64) (int32, error) {
	f, err := os.OpenFile(dev, os.O_RDWR, os.ModeDevice)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	r := &gpio_v2_line_request{
		offsets:  [gpio_v2_lines_max]uint32{line},
		consumer: [gpio_max_name_size]byte{'i', 'n', 'k', 'y', 0},
		config: gpio_v2_line_config{
			flags: flags,
		},
		num_lines: 1,
	}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(),
		gpio_v2_get_line_ioctl,
		uintptr(unsafe.Pointer(r)))
	if errno != 0 {
		return 0, fmt.Errorf("error requesting gpio line %v: %v", line, syscall.Errno(errno))
	}

	return r.fd, nil
}

func setGPIO(fd int32, val uint64) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd),
		gpio_v2_line_set_values_ioctl,
		uintptr(unsafe.Pointer(&gpio_v2_line_values{
			bits: 1 & val,
			mask: 1,
		})))
	if errno != 0 {
		return fmt.Errorf("error setting gpio line: %v", syscall.Errno(errno))
	}
	return nil
}
