FROM golang:latest AS build

COPY . /opt/build

RUN go install github.com/evanw/esbuild/cmd/esbuild@latest

RUN cd /opt/build; make all

FROM golang:latest

COPY --from=build /opt/build/build /opt/buildrone

EXPOSE 8062

CMD [ "/opt/buildrone/buildrone", "-config", "/config.ini", "-data", "/data" ]


