FROM mcr.microsoft.com/devcontainers/base:ubuntu-24.04

RUN apt-get update \
    && apt-get upgrade -y \
    && apt-get install -y --no-install-recommends make git jq ripgrep ca-certificates \
    && rm -rf /var/lib/apt/lists/*