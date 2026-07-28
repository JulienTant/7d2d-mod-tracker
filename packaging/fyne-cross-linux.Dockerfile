# fyne-cross 1.6.2's Linux image does not include EGL headers required by
# current GLFW/Fyne builds. Keep this image minimal and remove it once the
# upstream image includes libegl-dev for all target architectures.
FROM fyneio/fyne-cross-images:linux@sha256:da6dc85db54d161eaa45342d04e6eeb5aa388da9fcc3694b0fff932c7416b116

RUN set -eux; \
    apt-get update; \
    apt-get install -y -q --no-install-recommends \
        libegl-dev:amd64 \
        libegl-dev:i386 \
        libegl-dev:arm64 \
        libegl-dev:armhf \
    ; \
    apt-get clean; \
    rm -rf /var/lib/apt/lists/*
