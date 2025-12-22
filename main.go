// This Source Form is subject to the terms of the AQUA Software License, v. 1.0.
// Copyright (c) 2025 Aymeric Wibo

package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"runtime"
	"strings"

	"github.com/dhowden/tag"
	"github.com/hraban/opus"

	// "github.com/pion/opus/pkg/oggreader"
	"obiw.ac/aqua"
	"obiw.ac/aqua/wgpu"
)

const PATH = "private-idaho.opus"
const SAMPLE_RATE = 48000

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx := aqua.Init()

	if ctx == nil {
		panic("Failed to initialize AQUA context.")
	}

	descr := ctx.GetKosDescr()
	fmt.Printf("AQUA context initialized successfully: KOS v%d, %s.\n", descr.ApiVers, descr.Name)

	var iter *aqua.VdevIter

	// Get window VDEV.

	win_comp := ctx.WinInit()
	iter = win_comp.NewVdevIter()

	var found *aqua.VdevDescr

	for vdev := iter.Next(); vdev != nil; vdev = iter.Next() {
		fmt.Printf("Found window VDEV: %s (\"%s\", from \"%s\").\n", vdev.Spec, vdev.Human, vdev.VdriverHuman)
		found = vdev
	}

	if found == nil {
		panic("No window VDEV found.")
	}

	win_ctx := win_comp.Conn(found)

	// Get WebGPU VDEV.

	wgpu_comp := ctx.WgpuInit()
	iter = wgpu_comp.NewVdevIter()

	for vdev := iter.Next(); vdev != nil; vdev = iter.Next() {
		fmt.Printf("Found WebGPU VDEV: %s (\"%s\", from \"%s\").\n", vdev.Spec, vdev.Human, vdev.VdriverHuman)
		found = vdev
	}

	if found == nil {
		panic("No WebGPU VDEV found.")
	}

	wgpu_ctx := wgpu_comp.Conn(found)

	// Get UI VDEV.

	ui_comp := ctx.UiInit()
	iter = ui_comp.NewVdevIter()

	found = nil

	for vdev := iter.Next(); vdev != nil; vdev = iter.Next() {
		fmt.Printf("Found UI VDEV: %s (\"%s\", from \"%s\").\n", vdev.Spec, vdev.Human, vdev.VdriverHuman)
		found = vdev
	}

	if found == nil {
		panic("No UI VDEV found.")
	}

	ui_ctx := ui_comp.Conn(found)

	if ui_ctx.GetSupportedBackends()&aqua.UI_BACKEND_WGPU == 0 {
		panic("WebGPU UI backend is not supported.")
	}

	// Get audio VDEV.

	audio_comp := ctx.AudioInit()
	iter = audio_comp.NewVdevIter()

	var audio_vdev *aqua.VdevDescr = nil

	for vdev := iter.Next(); vdev != nil; vdev = iter.Next() {
		fmt.Printf("Found audio VDEV: %s (\"%s\", from \"%s\").\n", vdev.Spec, vdev.Human, vdev.VdriverHuman)

		if audio_vdev == nil || strings.HasPrefix(vdev.Human, "default") {
			audio_vdev = vdev
		}
	}

	if audio_vdev == nil {
		panic("No audio VDEV found.")
	}

	audio_ctx := audio_comp.Conn(audio_vdev)

	// Look for suitable audio config and create audio stream with it.

	configs := audio_ctx.GetConfigs()
	var chosen_config *aqua.AudioConfig = nil

	for _, config := range configs {
		if config.SampleFormat == aqua.AUDIO_SAMPLE_FORMAT_F32 && SAMPLE_RATE >= config.MinSampleRate && SAMPLE_RATE <= config.MaxBufSize {
			chosen_config = &config
		}
	}

	if chosen_config == nil {
		panic("Couldn't find satisfactory config.")
	}

	fmt.Println("Config sample format:", chosen_config.SampleFormat)
	fmt.Println("Config channels:", chosen_config.Channels)
	fmt.Println("Config sample rate range:", chosen_config.MinSampleRate, chosen_config.MaxSampleRate)
	fmt.Println("Config buffer size range:", chosen_config.MinBufSize, chosen_config.MaxBufSize)

	const RINGBUF_SEC = 20
	stream, err := audio_ctx.OpenStream(chosen_config.SampleFormat, 1, SAMPLE_RATE, 960, SAMPLE_RATE*RINGBUF_SEC)
	if err != nil {
		panic(err)
	}

	// Create window.

	win := win_ctx.Create()
	defer win.Destroy()

	// Read audio file.

	f, err := os.Open(PATH)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// Play song.

	// ogg, header, err := oggreader.NewWith(f)
	// if err != nil {
	// 	panic(err)
	// }

	in_stream, err := opus.NewStream(f)
	if err != nil {
		panic(err)
	}
	defer in_stream.Close()

	buf := make([]float32, 2*960)
	mono := make([]float32, 960)

	for {
		n, err := in_stream.ReadFloat32(buf)
		if err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}

		for i := 0; i < n; i++ {
			mono[i] = float32(math.Sqrt(2)/2) * (buf[i*2] + buf[i*2+1])
		}

		stream.Write(aqua.AudioBufferNewCount(mono, n))
	}

	/*
		streamer, format, err := mp3.Decode(f)
		if err != nil {
			panic(err)
		}
		defer streamer.Close()

		fmt.Println(format)

		buf := make([][2]float64, 4096)
		mono := make([]float64, 4096)

		for {
			n, ok := streamer.Stream(buf)
			if !ok {
				break
			}

			for i := 0; i < n; i++ {
				mono[i] = math.Sqrt(2) / 2 * (buf[i][0] + buf[i][1])
			}

			stream.Write(aqua.AudioBufferNew(mono))
		}
	*/

	_ = stream

	// Extract metadata.

	f.Seek(0, io.SeekStart)

	meta, err := tag.ReadFrom(f)
	if err != nil {
		panic(err)
	}

	fmt.Println("Title:", meta.Title())
	fmt.Println("Artist:", meta.Artist())
	fmt.Println("Album:", meta.Album())
	fmt.Println("Year:", meta.Year())

	img, _, err := image.Decode(bytes.NewReader(meta.Picture().Data))
	if err != nil {
		panic(err)
	}

	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)

	// Create a UI.

	ui := ui_ctx.Create()
	defer ui.Destroy()

	root := ui.GetRoot()
	root.AddText("text.title", meta.Title())
	root.AddText("text.paragraph", fmt.Sprintf("%s - %s", meta.Album(), meta.Artist()))

	root.AddDiv("").SetAttr("min_h", aqua.UiDim{}.Pixels(30))
	album_cover := root.AddDiv("")

	album_cover.SetAttr("min_w", aqua.UiDim{}.Pixels(300))
	album_cover.SetAttr("min_h", aqua.UiDim{}.Pixels(300))

	album_cover.SetAttr("bg", aqua.UiRaster{
		XRes: uint32(img.Bounds().Dx()),
		YRes: uint32(img.Bounds().Dy()),
		Data: rgba.Pix,
	})

	// TODO Next steps:
	// - [x] Fix creating windows from threads (because of callbacks in the Go win bindings?).
	// - [ ] I need to be able to set div min/max sizes through attrs. Use this opportunity to make attrs proper members?
	// - [x] Need root element to actually have size of window.
	// - [ ] Proper centered alignment of elements (for that div).
	// - [ ] Also allow this positioning in absolute terms.
	// - [x] Image attributes for background, which like just takes in a pointer and width/height/format.
	// - [x] Read image from MP3 already.
	// - [x] Also might as well already display metadata like title, artist, album, whatever.
	// - [ ] Keyboard input so can play/pause.
	// - [ ] Nice animations on the album cover and perhaps vinyl (though don't know if that should be hidden on pause or not).
	// - [ ] At this point we should work on getting the cool shader background working. Though that's gonna involve some though work for wgpu bindings in Go.
	// - [ ] Make sound?
	// - [ ] Mice! Buttons!
	// - [ ] Progress indicator.
	// - [ ] Cool album cover flipping thingy.
	// - [ ] Shadow.
	// - [ ] Done????

	// Set up WebGPU backend for UI.

	var state *aqua.UiWgpuEzState

	if state, err = ui.WgpuEzSetup(win, wgpu_ctx); err != nil {
		panic("UI WebGPU backend setup failed.")
	}

	wgpu.SetGlobalCtx(state.RawWgpuCtx)

	// Start window loop.

	x := 0.0
	var bg *Bg = nil

	win.RegisterRedrawCb(func() {
		state.Render() // TODO In order to use the same command encoder, we probably don't wanna use the ez methods.
		x += 0.01
		// album_cover.SetAttr("rot", float32(x))

		// Create background if it doesn't already exist.

		if bg == nil {
			dev := wgpu.CreateDeviceFromRaw(state.RawDevice())

			if bg, err = (Bg{}).New(&dev); err != nil {
				panic(err)
			}

			root.SetAttr("bg.wgpu_tex_view", bg.view.ToRaw())
		} else {
			dev := wgpu.CreateDeviceFromRaw(state.RawDevice())

			if err := bg.Render(&dev); err != nil {
				panic(err)
			}
		}
	})

	win.RegisterResizeCb(func(x_res, y_res uint32) {
		state.Resize(x_res, y_res)
	})

	win.Loop()
}
