// This Source Form is subject to the terms of the AQUA Software License, v. 1.0.
// Copyright (c) 2025 Aymeric Wibo

package main

import (
	_ "embed"
	"obiw.ac/aqua/wgpu"
)

type Bg struct {
	// Texture stuff.

	tex  *wgpu.Texture
	view *wgpu.TextureView

	// Pipeline stuff.

	shader            *wgpu.ShaderModule
	bind_group_layout *wgpu.BindGroupLayout
	pipeline_layout   *wgpu.PipelineLayout
	pipeline          *wgpu.RenderPipeline
}

const FORMAT = wgpu.TextureFormatRGBA8Unorm

//go:embed shader.wgsl
var shader_src string

func (Bg) New(d *wgpu.Device) (*Bg, error) {
	bg := &Bg{}

	if err := bg.create_tex(d); err != nil {
		return nil, err
	}

	if err := bg.create_pipeline(d); err != nil {
		return nil, err
	}

	return bg, nil
}

func (bg *Bg) Render(d *wgpu.Device) error {
	cmd_enc, err := d.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{
		Label: "Background command encoder",
	})
	if err != nil {
		return err
	}
	defer cmd_enc.Release()

	render_pass := cmd_enc.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "Background texture render pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:    bg.view,
				LoadOp:  wgpu.LoadOpClear,
				StoreOp: wgpu.StoreOpStore,
				ClearValue: wgpu.Color{
					R: 1.0,
					G: 0.0,
					B: 1.0,
					A: 0.5,
				},
			},
		},
	})
	defer render_pass.Release()

	render_pass.SetPipeline(bg.pipeline)
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

func (bg *Bg) create_tex(d *wgpu.Device) error {
	var err error

	if bg.tex, err = d.CreateTexture(&wgpu.TextureDescriptor{
		Label: "Background texture",
		Size: wgpu.Extent3D{
			Width:              800,
			Height:             600,
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
			{ // Colour #1.
				Binding:    2,
				Visibility: wgpu.ShaderStageFragment,
				Buffer: wgpu.BufferBindingLayout{
					Type: wgpu.BufferBindingTypeUniform,
				},
			},
			{ // Colour #2.
				Binding:    3,
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
		// BindGroupLayouts: []*wgpu.BindGroupLayout{
		// 	bg.bind_group_layout,
		// },
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
					Blend:     &wgpu.BlendStatePremultipliedAlphaBlending,
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

	return nil
}
