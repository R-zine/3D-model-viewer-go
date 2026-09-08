#version 300 es

precision highp float;

in vec2 vUv;
in vec3 vNormal;

uniform sampler2D diffuseTex;
uniform vec4 baseColorFactor;
uniform vec3 lightDir;
uniform float alphaCutoff;

out vec4 fragmentColor;

vec3 srgbToLinear(vec3 value) {
    return pow(value, vec3(2.2));
}

vec3 linearToSrgb(vec3 value) {
    return pow(max(value, vec3(0.0)), vec3(1.0 / 2.2));
}

void main() {
    vec4 textureColor = texture(diffuseTex, vUv);
    vec4 baseColor = vec4(
        srgbToLinear(textureColor.rgb) * baseColorFactor.rgb,
        textureColor.a * baseColorFactor.a
    );
    if (alphaCutoff >= 0.0 && baseColor.a < alphaCutoff) {
        discard;
    }

    vec3 normalDirection = normalize(gl_FrontFacing ? vNormal : -vNormal);
    vec3 lightDirection = normalize(lightDir);
    float diffuseStrength = max(dot(normalDirection, lightDirection), 0.0);
    vec3 litColor = baseColor.rgb * (0.3 + diffuseStrength);
    fragmentColor = vec4(linearToSrgb(litColor), baseColor.a);
}
