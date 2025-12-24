// This Source Form is subject to the terms of the AQUA Software License, v. 1.0.
// Copyright (c) 2025 Aymeric Wibo

struct VertOut {
	@builtin(position) pos: vec4f,
	@location(0) tex_coord: vec2f,
	@location(1) colour: vec3f,
};

const POSITIONS: array<vec2f, 6> = array(
	vec2(-1.0, -1.0), // 0
	vec2( 1.0, -1.0), // 1
	vec2(-1.0,  1.0), // 2

	vec2(-1.0,  1.0), // 2
	vec2( 1.0, -1.0), // 1
	vec2( 1.0,  1.0)  // 3
);

const COLOURS: array<vec3f, 6> = array<vec3f, 6>( // XXX Dunno why an explicit constructor is required here but not above...
	vec3(1.0, 0.0, 0.0),
	vec3(0.0, 1.0, 0.0),
	vec3(0.0, 0.0, 1.0),

	vec3(0.0, 0.0, 1.0),
	vec3(0.0, 1.0, 0.0),
	vec3(1.0, 1.0, 1.0)
);

@vertex
fn vert_main(@builtin(vertex_index) index: u32) -> VertOut {
	var out: VertOut;

	let pos = POSITIONS[index];
	out.tex_coord = pos.xy / 2. + .5;
	out.pos = vec4(pos, 0.0, 1.0);
	out.colour = COLOURS[index];

	return out;
}

struct FragOut {
	@location(0) colour: vec4f,
};

@group(0) @binding(0)
var t: texture_2d<f32>;
@group(0) @binding(1)
var s: sampler;

@fragment
fn frag_main(vert: VertOut) -> FragOut {
	var out: FragOut;
	out.colour = vec4(vert.colour, 1.0) * textureSample(t, s, vert.tex_coord);
	return out;
}
