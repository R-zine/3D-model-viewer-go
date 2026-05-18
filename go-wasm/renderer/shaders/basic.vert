attribute vec3 position;
attribute vec3 normal;

uniform mat4 model;
uniform mat4 view;
uniform mat4 projection;

varying vec3 vNormal;
varying vec3 vPosition;

void main() {

    vec4 worldPosition =
        model * vec4(position, 1.0);

    vPosition = worldPosition.xyz;

    vNormal =
        mat3(model) * normal;

    gl_Position =
        projection *
        view *
        worldPosition;
}