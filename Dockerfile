##################################################
#                Go Router Build
##################################################

FROM golang:1.26-alpine AS router-build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/xinghai-router \
    ./cmd/router

##################################################
#                Go Router Runtime
##################################################

FROM alpine:3.22 AS router


RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S router \
    && adduser -S -G router router

COPY --from=router-build \
    /out/xinghai-router \
    /usr/local/bin/xinghai-router

RUN chown router:router /usr/local/bin/xinghai-router
USER router
ENV TZ=UTC
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/xinghai-router"]

##################################################
#                Web Dependencies
##################################################

FROM node:22-alpine AS web-dependencies

WORKDIR /src/web

ENV PNPM_HOME=/pnpm
ENV PATH=$PNPM_HOME:$PATH

# 安装 pnpm
RUN npm install -g pnpm \
    --registry=https://registry.npmmirror.com
# 设置 pnpm 国内源
RUN pnpm config set registry https://registry.npmmirror.com
COPY web/package.json web/pnpm-lock.yaml ./
# pnpm store缓存
RUN pnpm config set dangerouslyAllowAllBuilds true
RUN --mount=type=cache,id=xinghai-pnpm-store,target=/pnpm/store \
    pnpm config set store-dir /pnpm/store \
    && pnpm install --frozen-lockfile

##################################################
#                Web Build
##################################################

FROM web-dependencies AS web-build

COPY web ./

RUN pnpm run build

##################################################
#                Web Runtime
##################################################

FROM node:22-alpine AS web

WORKDIR /app

ENV NODE_ENV=production
ENV HOST=0.0.0.0
ENV PORT=3000

COPY --from=web-build \
    /src/web/.output \
    ./.output

RUN addgroup -S web \
    && adduser -S -G web web \
    && chown -R web:web /app

USER web

EXPOSE 3000

CMD ["node",".output/server/index.mjs"]
