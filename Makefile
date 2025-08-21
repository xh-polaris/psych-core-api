.PHONY: start build wire update new clean

SERVICE_NAME := psych.core_api
MODULE_NAME := github.com/xh-polaris/psych-core-api

HANDLER_DIR := biz/adaptor/controller
MODEL_DIR := biz/application/dto
ROUTER_DIR := biz/adaptor/router

IDL_DIR ?= ../psych-idl
# 提取最后一个部分（如 "psych.core_api" → "core_api"）
MAIN_IDL_BASE := $(subst -,_,$(shell echo $(SERVICE_NAME) | awk -F '.' '{print $$NF}'))
# 组合成最终路径（如 "$(IDL_DIR)/core_api/core_api.proto"）
FULL_MAIN_IDL_PATH := $(IDL_DIR)/$(MAIN_IDL_BASE)/$(MAIN_IDL_BASE).proto
IDL_DIR := "$(IDL_DIR)"
FULL_MAIN_IDL_PATH := "$(FULL_MAIN_IDL_PATH)"
# 这里由于idl中未直接放置google相关文件, 所以必须把这个文件放置到下面这个文件夹中
IDL_OPTIONS := -I $(IDL_DIR) --idl $(FULL_MAIN_IDL_PATH)
OUTPUT_OPTIONS := --handler_dir $(HANDLER_DIR) --model_dir $(MODEL_DIR) --router_dir $(ROUTER_DIR)
EXTRA_OPTIONS := --pb_camel_json_tag=true --unset_omitempty=true

run:
	sh ./output/bootstrap.sh
build:
	sh ./build.sh
build_and_run:
	sh ./build.sh && sh ./output/bootstrap.sh
wire:
	wire ./provider
update:
	hz --verbose update $(IDL_OPTIONS) --mod $(MODULE_NAME) $(EXTRA_OPTIONS)
	@files=$$(find biz/application/dto -type f); \
	for file in $$files; do \
  	  sed -i  -e 's/func init\(\).*//' $$file; \
  	done
new:
	hz new $(IDL_OPTIONS) $(OUTPUT_OPTIONS) --service $(SERVICE_NAME) --mod $(MODULE_NAME) $(EXTRA_OPTIONS)
clean:
	rm -r ./output
