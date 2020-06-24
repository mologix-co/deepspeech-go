package audio

import (
	"fmt"
	"io"
	"testing"
)

func TestBuffer_Write(t *testing.T) {
	reader, err := OpenWavFile("recording.wav", Ptime10)
	if err != nil {
		t.Fatal(err)
	}
	buffer := NewBuffer(reader, 100)

	go func() {
		defer reader.Close()
		count := 0
		for {
			frame, err := reader.ReadFrame()
			if err != nil {
				if len(frame) > 0 {
					_ = buffer.Write(frame)
					count++
				}
				_ = buffer.WriteFinal()
				return
			}

			count++
			err = buffer.WriteBlocking(frame)
			if err != nil {
				if err != io.ErrClosedPipe {
					t.Fatal(err)
				}
				return
			}
		}
	}()

	count := 0
	for {
		frame, err := buffer.ReadFrame()
		if err != nil {
			if err == io.EOF {
				fmt.Println("EOF")
				break
			}
			t.Fatal(err)
		}

		_ = frame
		fmt.Println(count)
		count++
	}
	_ = buffer.Close()
}
