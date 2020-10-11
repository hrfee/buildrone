FROM golang:latest AS build

COPY . /opt/build

RUN GO111MODULE=on go get github.com/evanw/esbuild/cmd/esbuild

RUN cd /opt/build; make all

FROM golang:latest

COPY --from=build /opt/build/build /opt/buildrone

EXPOSE 8062

CMD [ "/opt/buildrone/buildrone", "-config", "/config.ini", "-data", "/data" ]


