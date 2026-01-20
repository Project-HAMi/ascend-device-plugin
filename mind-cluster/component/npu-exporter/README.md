# NPU-Exporter

# 组件介绍


Prometheus（普罗米修斯）是一个开源的系统监测和警报工具包，Exporter就是专门为Prometheus提供数据源的组件。由于Prometheus社区的活跃和大量的使用，已经有很多厂商或者服务提供了Exporter，如Prometheus官方的Node Exporter，MySQL官方出的MySQL Server Exporter和NVIDA的NVIDIA GPU Exporter。这些Exporter负责将特定监测对象的指标，转成Prometheus能够识别的数据格式，供Prometheus集成。NPU-Expoter是华为自研的专门收集华为NPU各种监测信息和指标，并封装成Prometheus专用数据格式的一个服务组件。


# 编译NPU-Exporter

1. 通过git拉取源码，获得npu-exporter。

   示例：Npu-Exporter源码放在/home/mind-cluster/component/npu-exporter目录下

2.  执行以下命令，进入Npu-Exporter构建目录，执行构建脚本，在“output“目录下生成二进制npu-exporter、yaml文件和Dockerfile等文件。

    **cd** _/home/mind-cluster/component/_**npu-exporter/build/**

    **chmod +x build.sh**

    **./build.sh**

3.  执行以下命令，查看**output**生成的软件列表。

    **ll** _/home/mind-cluster/component/_**npu-exporter/output**

    ```
    drwxr-xr-x  2 root root     4096 Feb 23 07:10 .
    drwxr-xr-x 10 root root     4096 Feb 23 07:10 ..
    -r--------  1 root root      623 Feb 23 07:10 Dockerfile
    -r--------  1 root root      623 Feb 23 07:10 Dockerfile-310P-1usoc
    -r--------  1 root root      623 Feb 23 07:10 metricConfiguration.json
    -r-x------  1 root root 25481072 Feb 23 07:10 npu-exporter
    -r--------  1 root root     3438 Feb 23 07:10 npu-exporter-310P-1usoc-v6.0.0.yaml
    -r--------  1 root root     3438 Feb 23 07:10 npu-exporter-v6.0.0.yaml
    -r--------  1 root root      623 Feb 23 07:10 pluginConfiguration.json
    -r-x------  1 root root     2579 Feb 23 07:10 run_for_310P_1usoc.sh
    ```

# 说明

1. 当前Npu-Exporter仅支持http启动，如果需要使用https启动，请自行完成代码修改并适配Prometheus