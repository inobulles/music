// This Source Form is subject to the terms of the AQUA Software License, v. 1.0.
// Copyright (c) 2025 Aymeric Wibo

package main

import (
	_ "embed"
	"image"
	"image/draw"
	"os"
	"unsafe"

	"obiw.ac/aqua/wgpu"
)

type SwirlData struct {
	Progression float32
}

type Bg struct {
	// Swirl texture stuff.

	swirl_tex     *wgpu.Texture
	swirl_view    *wgpu.TextureView
	swirl_sampler *wgpu.Sampler

	// Render attachment texture stuff.

	tex  *wgpu.Texture
	view *wgpu.TextureView

	// Pipeline stuff.

	shader            *wgpu.ShaderModule
	bind_group_layout *wgpu.BindGroupLayout
	pipeline_layout   *wgpu.PipelineLayout
	pipeline          *wgpu.RenderPipeline
	bind_group        *wgpu.BindGroup

	// Swirl data.

	swirl_data         SwirlData
	swirl_data_uniform *wgpu.Buffer
}

const FORMAT = wgpu.TextureFormatRGBA8Unorm

//go:embed shader.wgsl
var shader_src string

func (Bg) New(d *wgpu.Device) (*Bg, error) {
	bg := &Bg{}

	if err := bg.create_swirl(d); err != nil {
		return nil, err
	}

	if err := bg.create_tex(d); err != nil {
		return nil, err
	}

	if err := bg.create_pipeline(d); err != nil {
		return nil, err
	}

	return bg, nil
}

func (bg *Bg) Render(d *wgpu.Device, swirl_progression float64) error {
	cmd_enc, err := d.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{
		Label: "Background command encoder",
	})
	if err != nil {
		return err
	}
	defer cmd_enc.Release()

	queue := d.GetQueue()

	bg.swirl_data.Progression = float32(swirl_progression)
	const size = uint64(unsafe.Sizeof(bg.swirl_data))
	queue.WriteBuffer(bg.swirl_data_uniform, 0, wgpu.ToBytes((*(*[uint64(unsafe.Sizeof(bg.swirl_data))]byte)(unsafe.Pointer(&bg.swirl_data)))[:])) // lol

	render_pass := cmd_enc.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "Background texture render pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:    bg.view,
				LoadOp:  wgpu.LoadOpClear,
				StoreOp: wgpu.StoreOpStore,
				ClearValue: wgpu.Color{
					R: 0.0,
					G: 0.0,
					B: 0.0,
					A: 0.0,
				},
			},
		},
	})
	defer render_pass.Release()

	render_pass.SetPipeline(bg.pipeline)
	render_pass.SetBindGroup(0, bg.bind_group, nil)
	render_pass.Draw(6, 1, 0, 0)
	render_pass.End()

	cmd_buf, err := cmd_enc.Finish(&wgpu.CommandBufferDescriptor{
		Label: "Background command buffer",
	})
	if err != nil {
		return err
	}
	defer cmd_buf.Release()

	d.GetQueue().Submit(cmd_buf)

	return nil
}

func (bg *Bg) create_swirl(d *wgpu.Device) error {
	f, err := os.Open("swirl.png")
	if err != nil {
		return err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}

	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)

	// Create texture for swirl.

	w := uint32(img.Bounds().Dx())
	h := uint32(img.Bounds().Dy())

	size := wgpu.Extent3D{
		Width:              w,
		Height:             h,
		DepthOrArrayLayers: 1,
	}

	if bg.swirl_tex, err = d.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "Background swirl texture",
		Size:          size,
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        wgpu.TextureFormatRGBA8Unorm,
		Usage:         wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopyDst,
	}); err != nil {
		return err
	}

	if err := d.GetQueue().WriteTexture(
		bg.swirl_tex.AsImageCopy(),
		rgba.Pix,
		&wgpu.TexelCopyBufferLayout{
			Offset:       0,
			BytesPerRow:  4 * w,
			RowsPerImage: h,
		},
		&size,
	); err != nil {
		return err
	}

	if bg.swirl_view, err = bg.swirl_tex.CreateView(nil); err != nil {
		return err
	}

	if bg.swirl_sampler, err = d.CreateSampler(&wgpu.SamplerDescriptor{
		AddressModeU:  wgpu.AddressModeClampToEdge,
		AddressModeV:  wgpu.AddressModeClampToEdge,
		AddressModeW:  wgpu.AddressModeClampToEdge,
		MagFilter:     wgpu.FilterModeLinear,
		MinFilter:     wgpu.FilterModeLinear,
		MipmapFilter:  wgpu.MipmapFilterModeLinear,
		MaxAnisotropy: 1,
	}); err != nil {
		return err
	}

	return nil
}

func (bg *Bg) create_tex(d *wgpu.Device) error {
	var err error

	if bg.tex, err = d.CreateTexture(&wgpu.TextureDescriptor{
		Label: "Background texture",
		Size: wgpu.Extent3D{
			Width:              1920,
			Height:             1080,
			DepthOrArrayLayers: 1,
		},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        FORMAT,
		Usage:         wgpu.TextureUsageTextureBinding | wgpu.TextureUsageRenderAttachment,
	}); err != nil {
		return err
	}

	if bg.view, err = bg.tex.CreateView(nil); err != nil {
		return err
	}

	return nil
}

func (bg *Bg) create_pipeline(d *wgpu.Device) error {
	var err error

	if bg.shader, err = d.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: "Background shader module",
		WGSLSource: &wgpu.ShaderSourceWGSL{
			Code: shader_src,
		},
	}); err != nil {
		return err
	}

	if bg.bind_group_layout, err = d.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "Background bind group layout",
		Entries: []wgpu.BindGroupLayoutEntry{
			{ // Texture.
				Binding:    0,
				Visibility: wgpu.ShaderStageFragment,
				Texture: wgpu.TextureBindingLayout{
					Multisampled:  false,
					ViewDimension: wgpu.TextureViewDimension2D,
					SampleType:    wgpu.TextureSampleTypeFloat,
				},
			},
			{ // Sampler.
				Binding:    1,
				Visibility: wgpu.ShaderStageFragment,
				Sampler: wgpu.SamplerBindingLayout{
					Type: wgpu.SamplerBindingTypeFiltering,
				},
			},
			{ // Other data.
				Binding:    2,
				Visibility: wgpu.ShaderStageFragment,
				Buffer: wgpu.BufferBindingLayout{
					Type: wgpu.BufferBindingTypeUniform,
				},
			},
		},
	}); err != nil {
		return err
	}

	if bg.pipeline_layout, err = d.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label: "Background pipeline layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{
			bg.bind_group_layout,
		},
	}); err != nil {
		return err
	}

	if bg.pipeline, err = d.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  "Background render pipeline",
		Layout: bg.pipeline_layout,
		Primitive: wgpu.PrimitiveState{
			Topology:         wgpu.PrimitiveTopologyTriangleList,
			StripIndexFormat: wgpu.IndexFormatUndefined,
			FrontFace:        wgpu.FrontFaceCCW,
			CullMode:         wgpu.CullModeNone,
		},
		Vertex: wgpu.VertexState{
			Module:     bg.shader,
			EntryPoint: "vert_main",
		},
		Fragment: &wgpu.FragmentState{
			Module:     bg.shader,
			EntryPoint: "frag_main",
			Targets: []wgpu.ColorTargetState{
				{
					Format:    FORMAT,
					Blend:     &wgpu.BlendStateAlphaBlending,
					WriteMask: wgpu.ColorWriteMaskAll,
				},
			},
		},
		Multisample: wgpu.MultisampleState{
			Count:                  1,
			Mask:                   0xFFFFFFFF,
			AlphaToCoverageEnabled: false,
		},
	}); err != nil {
		return err
	}

	println(uint64(unsafe.Sizeof(bg.swirl_data)))

	if bg.swirl_data_uniform, err = d.CreateBuffer(&wgpu.BufferDescriptor{
		Size:  uint64(unsafe.Sizeof(bg.swirl_data)) * 4,
		Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst,
	}); err != nil {
		return err
	}

	if bg.bind_group, err = d.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout: bg.bind_group_layout,
		Entries: []wgpu.BindGroupEntry{
			{
				Binding:     0,
				TextureView: bg.swirl_view,
			},
			{
				Binding: 1,
				Sampler: bg.swirl_sampler,
			},
			{
				Binding: 2,
				Buffer:  bg.swirl_data_uniform,
				Size:    wgpu.WholeSize,
			},
		},
	}); err != nil {
		return err
	}

	return nil
}
