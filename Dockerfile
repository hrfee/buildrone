FROM golang:latest AS build

RUN curl -fsSL https://deb.nodesource.com/setup_22.x -o nodesource_setup.sh && chmod +x nodesource_setup.sh && ./nodesource_setup.sh \
    && apt-get update -y && apt-get install nodejs -y && npm i -G npm

COPY . /opt/build

RUN cd /opt/build; npm i; make

FROM golang:latest

COPY --from=build /opt/build/build /opt/buildrone

EXPOSE 8062

CMD [ "/opt/buildrone/buildrone", "-config", "/config.ini", "-data", "/data" ]


