package container

import (
	"fmt"
	"net"
	"reflect"
	"runtime"
	"strings"
)

func ipOf(hostname string) net.IP {
	ip, err := net.LookupIP(hostname)
	if err != nil {
		failed("to lookup ip of container:", err)
	}
	return ip[0]
}

// funcName source: https://stackoverflow.com/a/7053871
func funcName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func callerFuncName() string {
	return getFrame(2).Function
}

// getFrame source: https://stackoverflow.com/a/35213181
func getFrame(skipFrames int) runtime.Frame {
	// We need the frame at index skipFrames+2, since we never want runtime.Callers and getFrame
	targetFrameIndex := skipFrames + 2

	// Set size to targetFrameIndex+2 to ensure we have room for one more caller than we need
	programCounters := make([]uintptr, targetFrameIndex+2)
	n := runtime.Callers(0, programCounters)

	frame := runtime.Frame{Function: "unknown"}
	if n > 0 {
		frames := runtime.CallersFrames(programCounters[:n])
		for more, frameIndex := true, 0; more && frameIndex <= targetFrameIndex; frameIndex++ {
			var frameCandidate runtime.Frame
			frameCandidate, more = frames.Next()
			if frameIndex == targetFrameIndex {
				frame = frameCandidate
			}
		}
	}

	return frame
}

func fail(message string) {
	panic(message)
}

func failed(message string, err error) {
	panic(fmt.Sprint("failed ", strings.TrimSpace(message)+" ", err))
}
