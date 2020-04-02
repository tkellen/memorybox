FROM alpine:latest
WORKDIR /root
COPY memorybox .
CMD ["./memorybox"]