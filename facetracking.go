package main

import (
	"bufio"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"os"
	"os/exec"
	"strconv"
	"time"

	"gocv.io/x/gocv"

	"gobot.io/x/gobot"
	"gobot.io/x/gobot/platforms/dji/tello"
	"gobot.io/x/gobot/platforms/joystick"
	"gobot.io/x/gobot/platforms/keyboard"
)

const maxJoyVal = 32768
const frameSize = frameX * frameY * 3
const frameX = 1280 // 400
const frameY = 720  // 300

var drone = tello.NewDriver("8890")
var window = gocv.NewWindow("Tello")

var ffmpeg = exec.Command("ffmpeg", "-hwaccel", "auto", "-hwaccel_device", "opencl", "-i", "pipe:0",
	"-pix_fmt", "bgr24", "-s", strconv.Itoa(frameX)+"x"+strconv.Itoa(frameY), "-f", "rawvideo", "pipe:1")
var ffmpegIn, _ = ffmpeg.StdinPipe()
var ffmpegOut, _ = ffmpeg.StdoutPipe()

var joyAdaptor = joystick.NewAdaptor()
var stick = joystick.NewDriver(joyAdaptor, "dualshock4")
var keys = keyboard.NewDriver()

var flightData *tello.FlightData
var tracking = false
var detectSize = false
var distTolerance = 0.05 * dist(0, 0, frameX, frameY)
var hasNCS = false

func init() {
	handleJoystick()
	handleKeyboard()
	go func() {
		if err := ffmpeg.Start(); err != nil {
			fmt.Println(err)
			return
		}

		drone.On(tello.FlightDataEvent, func(data interface{}) {
			flightData = data.(*tello.FlightData)
		})

		drone.On(tello.ConnectedEvent, func(data interface{}) {
			fmt.Println("Connected")
			drone.StartVideo()
			drone.SetVideoEncoderRate(tello.VideoBitRateAuto)
			drone.SetExposure(0)
			gobot.Every(100*time.Millisecond, func() {
				drone.StartVideo()
			})
		})

		drone.On(tello.VideoFrameEvent, func(data interface{}) {
			pkt := data.([]byte)
			if _, err := ffmpegIn.Write(pkt); err != nil {
				fmt.Println(err)
			}
		})

		robot := gobot.NewRobot("tello",
			[]gobot.Connection{},
			[]gobot.Connection{joyAdaptor},
			[]gobot.Device{stick},
			[]gobot.Device{drone},
			[]gobot.Device{keys},
		)

		robot.Start()
	}()
}

// readDescriptions reads the descriptions from a file
// and returns a slice of its lines.
func readDescriptions(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("How to run:\ngo run facetracking.go [protofile] [modelfile]")
		return
	}
	proto := ""
	model := ""
	/*
		graphFileName := ""
		deviceID := 0
		var descriptions []string
		var err error
	*/
	mode := os.Args[1]
	if mode == "normal" {
		proto = os.Args[2]
		model = os.Args[3]
	} else {
		//		graphFileName = os.Args[2]
		//		descriptions, err = readDescriptions(os.Args[3])
	}

	net := gocv.ReadNetFromCaffe(proto, model)
	if net.Empty() {
		fmt.Printf("Error reading network model from : %v %v\n", proto, model)
		return
	}
	defer net.Close()

	green := color.RGBA{0, 255, 0, 0}

	if net.Empty() {
		fmt.Printf("Error reading network model from : %v %v\n", proto, model)
		return
	}
	defer net.Close()

	/*
		// get name of NCS stick
		res, name := ncs.GetDeviceName(deviceID)
		if res != ncs.StatusOK {
			fmt.Printf("NCS Error: %v\n", res)
			return
		}

		fmt.Println("NCS: " + name)

		// open NCS device
		fmt.Println("Opening NCS device " + name + "...")
		status, s := ncs.OpenDevice(name)
		if status != ncs.StatusOK {
			fmt.Printf("NCS Error: %v\n", status)
			return
		}
		defer s.CloseDevice()

		// load precompiled graph file in NCS format
		data, err := ioutil.ReadFile(graphFileName)
		if err != nil {
			fmt.Println("Error opening graph file:", err)
			return
		}
		// allocate graph on NCS stick
		fmt.Println("Allocating graph...")
		allocateStatus, graph := s.AllocateGraph(data)
		if allocateStatus != ncs.StatusOK {
			fmt.Printf("NCS Error: %v\n", allocateStatus)
			return
		}
		defer graph.DeallocateGraph()
		hasNCS = true
	*/
	refDistance := float64(0)
	detected := false
	left := float32(0)
	top := float32(0)
	right := float32(0)
	bottom := float32(0)
	/*
		resized := gocv.NewMat()
		defer resized.Close()

		fp32Image := gocv.NewMat()
		defer fp32Image.Close()

		statusColor := color.RGBA{0, 255, 0, 0}
	*/
	for {
		buf := make([]byte, frameSize)
		if _, err := io.ReadFull(ffmpegOut, buf); err != nil {
			fmt.Println(err)
			continue
		}
		img := gocv.NewMatFromBytes(frameY, frameX, gocv.MatTypeCV8UC3, buf)
		if img.Empty() {
			continue
		}
		W := float32(img.Cols())
		H := float32(img.Rows())
		/*
			if hasNCS {

				// convert image to format needed by NCS
				gocv.Resize(img, &resized, image.Pt(224, 224), 0, 0, gocv.InterpolationDefault)
				resized.ConvertTo(&fp32Image, gocv.MatTypeCV32F)
				fp16Blob := fp32Image.ConvertFp16()

				// load image tensor into graph on NCS stick
				loadStatus := graph.LoadTensor(fp16Blob.ToBytes())
				if loadStatus != ncs.StatusOK {
					fmt.Println("Error loading tensor data:", loadStatus)
					return
				}

				// get result from NCS stick in fp16 format
				resultStatus, data := graph.GetResult()
				if resultStatus != ncs.StatusOK {
					fmt.Println("Error getting results:", resultStatus)
					return
				}

				// convert results from fp16 back to float32
				fp16Results := gocv.NewMatFromBytes(1, len(data)/2, gocv.MatTypeCV16S, data)
				results := fp16Results.ConvertFp16()

				// determine the most probable classification
				_, maxVal, _, maxLoc := gocv.MinMaxLoc(results)

				// display classification
				info := fmt.Sprintf("description: %v, maxVal: %v", descriptions[maxLoc.X], maxVal)
				gocv.PutText(&img, info, image.Pt(10, 20), gocv.FontHersheyPlain, 1.2, statusColor, 2)

				fp16Blob.Close()

			} else {
		*/
		blob := gocv.BlobFromImage(img, 1.0, image.Pt(128, 96), gocv.NewScalar(104.0, 177.0, 123.0, 0), false, false)
		defer blob.Close()

		net.SetInput(blob, "data")

		detBlob := net.Forward("detection_out")
		defer detBlob.Close()

		detections := gocv.GetBlobChannel(detBlob, 0, 0)
		defer detections.Close()

		for r := 0; r < detections.Rows(); r++ {
			confidence := detections.GetFloatAt(r, 2)
			if confidence < 0.5 {
				continue
			}

			left = detections.GetFloatAt(r, 3) * W
			top = detections.GetFloatAt(r, 4) * H
			right = detections.GetFloatAt(r, 5) * W
			bottom = detections.GetFloatAt(r, 6) * H

			left = min(max(0, left), W-1)
			right = min(max(0, right), W-1)
			bottom = min(max(0, bottom), H-1)
			top = min(max(0, top), H-1)

			rect := image.Rect(int(left), int(top), int(right), int(bottom))
			gocv.Rectangle(&img, rect, green, 3)
			detected = true
		}
		//		}

		window.IMShow(img)
		if window.WaitKey(10) >= 0 {
			break
		}

		if !tracking || !detected {
			continue
		}

		if detectSize {
			detectSize = false
			refDistance = dist(left, top, right, bottom)
		}

		distance := dist(left, top, right, bottom)

		if right < W/2 {
			drone.CounterClockwise(50)
		} else if left > W/2 {
			drone.Clockwise(50)
		} else {
			drone.Clockwise(0)
		}

		if top < H/10 {
			drone.Up(25)
		} else if bottom > H-H/10 {
			drone.Down(25)
		} else {
			drone.Up(0)
		}

		if distance < refDistance-distTolerance {
			drone.Forward(20)
		} else if distance > refDistance+distTolerance {
			drone.Backward(20)
		} else {
			drone.Forward(0)
		}
	}
}

func dist(x1, y1, x2, y2 float32) float64 {
	return math.Sqrt(float64((x2-x1)*(x2-x1) + (y2-y1)*(y2-y1)))
}

func min(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func handleKeyboard() {
	keys.On(keyboard.Key, func(data interface{}) {
		key := data.(keyboard.KeyEvent)
		switch key.Key {
		case keyboard.O:
			toggleTracking()
		case keyboard.B:
			battaryInfo()
		case keyboard.T:
			takeOff()
		case keyboard.L:
			landing()
		case keyboard.A:
			headCounterClockwise()
		case keyboard.D:
			headClockwise()
		case keyboard.W:
			moveForward()
		case keyboard.S:
			moveBackward()
		case keyboard.Q:
			shiftLeft()
		case keyboard.E:
			shiftRight()
		case keyboard.Z:
			flyHigher()
		case keyboard.X:
			flyLower()
		default:
			fmt.Println("keyboard event!", key, key.Char)
		}
	})

}

func toggleTracking() {
	drone.Forward(0)
	drone.Up(0)
	drone.Clockwise(0)
	tracking = !tracking
	if tracking {
		detectSize = true
		println("tracking")
	} else {
		detectSize = false
		println("not tracking")
	}
}

func battaryInfo() {
	fmt.Println("battery:", flightData.BatteryPercentage)
}

func takeOff() {
	drone.TakeOff()
	println("Takeoff")
}
func landing() {
	drone.Land()
	println("Land")
}
func moveForward() {
	drone.Forward(1)
}
func moveBackward() {
	drone.Backward(1)
}
func flyHigher() {
	drone.Up(1)
}
func flyLower() {
	drone.Down(1)
}
func headCounterClockwise() {
	drone.CounterClockwise(1)
}
func headClockwise() {
	drone.Clockwise(1)
}
func shiftLeft() {
	drone.Left(1)
}
func shiftRight() {
	drone.Right(1)
}
func handleJoystick() {
	stick.On(joystick.CirclePress, func(data interface{}) {
		toggleTracking()
	})
	stick.On(joystick.SquarePress, func(data interface{}) {
		battaryInfo()
	})
	stick.On(joystick.TrianglePress, func(data interface{}) {
		takeOff()
	})
	stick.On(joystick.XPress, func(data interface{}) {
		landing()
	})
	stick.On(joystick.RightY, func(data interface{}) {
		val := float64(data.(int16))
		if val >= 0 {
			drone.Backward(tello.ValidatePitch(val, maxJoyVal))
		} else {
			drone.Forward(tello.ValidatePitch(val, maxJoyVal))
		}
	})
	stick.On(joystick.RightX, func(data interface{}) {
		val := float64(data.(int16))
		if val >= 0 {
			drone.Right(tello.ValidatePitch(val, maxJoyVal))
		} else {
			drone.Left(tello.ValidatePitch(val, maxJoyVal))
		}
	})
	stick.On(joystick.LeftY, func(data interface{}) {
		val := float64(data.(int16))
		if val >= 0 {
			drone.Down(tello.ValidatePitch(val, maxJoyVal))
		} else {
			drone.Up(tello.ValidatePitch(val, maxJoyVal))
		}
	})
	stick.On(joystick.LeftX, func(data interface{}) {
		val := float64(data.(int16))
		if val >= 0 {
			drone.Clockwise(tello.ValidatePitch(val, maxJoyVal))
		} else {
			drone.CounterClockwise(tello.ValidatePitch(val, maxJoyVal))
		}
	})
}
