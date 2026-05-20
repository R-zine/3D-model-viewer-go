precision mediump float;

varying vec2 vUv;
varying vec3 vNormal;

uniform sampler2D diffuseTex;
uniform vec3 lightDir;

void main() {

    vec3 N = normalize(vNormal);
    vec3 L = normalize(lightDir);

    float diff = max(dot(N, L), 0.0);

   vec2 flippedUv = vec2(
    vUv.x,
    1.0 - vUv.y
);

vec3 tex =
    texture2D(
        diffuseTex,
        flippedUv
    ).rgb;

vec3 ambient = 0.3 * tex;
vec3 diffuse = diff * tex;

gl_FragColor = vec4(ambient + diffuse, 1.0);
}