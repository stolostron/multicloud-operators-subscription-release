FROM scratch

COPY multicloud-operators-subscription-release .
CMD ["./multicloud-operators-subscription-release"]
