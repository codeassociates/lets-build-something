FROM node:22-alpine AS build
WORKDIR /src

COPY package.json package-lock.json ./
# esbuild's postinstall fetches its platform binary; npm blocks install scripts
# by default in recent versions, so it is allowed explicitly here.
RUN npm ci --ignore-scripts && npm rebuild esbuild

COPY . .
RUN npm run build

# nginx serves the built assets and forwards /api to the backend service, so
# the browser only ever talks to one origin and the session cookie is
# first-party in every environment.
FROM nginx:1.29-alpine
COPY --from=build /src/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
