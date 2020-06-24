// +build linux

package main

/*
#cgo darwin LDFLAGS: -Wl,-rpath,./ -L./linux -ldeepspeech
*/
import "C"
