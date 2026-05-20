attribute vec3 position;
attribute vec3 normal;
attribute vec2 uv;

uniform mat4 model;
uniform mat4 view;
uniform mat4 projection;

varying vec2 vUv;
varying vec3 vNormal;


void main() {

    vec4 worldPos = model * vec4(position, 1.0);

    vUv = uv;

    vNormal = normalize(mat3(model) * normal);

    gl_Position = projection * view * worldPos;
}