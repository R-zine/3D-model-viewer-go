precision mediump float;

varying vec3 vNormal;
varying vec3 vPosition;

void main() {

    vec3 lightPosition =
        vec3(0.7, 0.8, 1.0);

    vec3 lightDirection =
        normalize(
            lightPosition - vPosition
        );

    float diffuse =
        max(
            dot(
                normalize(vNormal),
                lightDirection
            ),
            0.0
        );

    float distance =
        length(
            lightPosition - vPosition
        );

    float attenuation =
        2.5 / (distance * distance);

    float ambient = 0.1;

    vec3 baseColor =
        vec3(0.2, 0.8, 1.0);

    vec3 color =
        baseColor *
        (ambient + diffuse * attenuation);

    gl_FragColor =
        vec4(color, 1.0);
}