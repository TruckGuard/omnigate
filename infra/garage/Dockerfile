FROM dxflrs/garage:v1.0.0 as garage
FROM alpine:latest
COPY --from=garage /garage /usr/local/bin/garage
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
