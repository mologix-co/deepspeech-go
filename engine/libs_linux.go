// +build linux

package main

/*
#cgo linux LDFLAGS: -Wl,-rpath,./ -L./linux -ldeepspeech
*/
import "C"
