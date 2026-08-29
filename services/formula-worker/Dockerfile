FROM docker.1ms.run/library/alpine:latest

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk --no-cache add ca-certificates tzdata wget python3 py3-pip libstdc++ && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

ENV TZ=Asia/Shanghai \
    FORMULA_WORKER_HOST=0.0.0.0 \
    FORMULA_WORKER_PORT=8712

WORKDIR /app
COPY services/formula-worker ./formula-worker

RUN apk --no-cache add --virtual .hqchart-build g++ python3-dev zlib-dev linux-headers && \
    python3 /app/formula-worker/install_hqchartpy2.py && \
    apk del .hqchart-build && \
    chmod +x /app/formula-worker/worker.py && \
    chown -R appuser:appuser /app

USER appuser
EXPOSE 8712

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 -O /dev/null http://localhost:8712/health || exit 1

CMD ["python3", "/app/formula-worker/worker.py"]
