FROM golang:latest AS build

COPY . /opt/build

RUN cd /opt/build; npm i; make

FROM golang:latest

COPY --from=build /opt/build/build /opt/buildrone

EXPOSE 8062

CMD [ "/opt/buildrone/buildrone", "-config", "/config.ini", "-data", "/data" ]


