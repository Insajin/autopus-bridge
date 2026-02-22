FROM chromedp/headless-shell:latest

# non-root 사용자 생성
RUN groupadd -r sandbox && useradd -r -g sandbox -G audio,video sandbox \
    && mkdir -p /home/sandbox && chown sandbox:sandbox /home/sandbox

# 한국어 폰트 (REQ-C4-05)
RUN apt-get update && apt-get install -y --no-install-recommends \
    fonts-noto-cjk \
    && rm -rf /var/lib/apt/lists/*

# Health check: CDP 연결 확인
HEALTHCHECK --interval=5s --timeout=3s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9222/json/version || exit 1

USER sandbox
EXPOSE 9222

ENTRYPOINT ["headless-shell", \
    "--disable-gpu", \
    "--disable-software-rasterizer", \
    "--disable-dev-shm-usage", \
    "--disable-setuid-sandbox", \
    "--remote-debugging-address=0.0.0.0", \
    "--remote-debugging-port=9222"]
