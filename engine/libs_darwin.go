// +build darwin

package main

/*
#cgo darwin LDFLAGS: -Wl,-rpath,./ -L./mac -ldeepspeech
*/
import "C"
