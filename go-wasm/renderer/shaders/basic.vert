#version 300 es

in vec3 position;
in vec3 normal;
in vec2 uv;

uniform mat4 model;
uniform mat3 normalMatrix;
uniform mat4 view;
uniform mat4 projection;

out vec2 vUv;
out vec3 vNormal;

void main() {
    vec4 worldPosition = model * vec4(position, 1.0);
    vUv = uv;
    vNormal = normalize(normalMatrix * normal);
    gl_Position = projection * view * worldPosition;
}
