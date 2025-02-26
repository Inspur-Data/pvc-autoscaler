# 定义变量
BUILD_ARCH ?= amd64
DOCKER_IMAGE ?= cronpva-operator
DOCKER_TAG ?= latest
GOPROXY ?= https://goproxy.cn

# 构建 Docker 镜像
build:
	docker build --build-arg BUILD_ARCH=${BUILD_ARCH} --build-arg GOPROXY=${GOPROXY} -t ${DOCKER_IMAGE}:${DOCKER_TAG} .

# 运行 Docker 容器
run:
	docker run --rm ${DOCKER_IMAGE}:${DOCKER_TAG}

# 清理构建生成的文件
clean:
	rm -rf _output

# 默认目标
.PHONY: build run clean