FROM node:22
WORKDIR /src
COPY frontend /src
RUN npm install && npm run build
ENTRYPOINT [ "sleep", "1" ]