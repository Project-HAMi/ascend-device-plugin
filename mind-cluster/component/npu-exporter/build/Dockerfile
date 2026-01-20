FROM ubuntu:22.04

RUN useradd -d /home/HwHiAiUser -u 1000 -m -s /usr/sbin/nologin HwHiAiUser &&\
    usermod root -s /usr/sbin/nologin

COPY ./npu-exporter  /usr/local/bin/
COPY ./metricConfiguration.json  /usr/local/metricConfiguration.json
COPY ./pluginConfiguration.json  /usr/local/pluginConfiguration.json

RUN chown root:root /usr/local/bin/npu-exporter  &&\
    chmod 750 -R /home/HwHiAiUser &&\
    chmod 550 /usr/local/bin/ &&\
    chmod 500 /usr/local/bin/npu-exporter &&\
    chmod 440 /usr/local/metricConfiguration.json &&\
    chmod 440 /usr/local/pluginConfiguration.json &&\
    echo 'umask 027' >> /etc/profile && \
    echo 'source /etc/profile' >> ~/.bashrc
ENV LD_LIBRARY_PATH=/usr/local/Ascend/driver/lib64/driver:/usr/local/Ascend/driver/lib64/common:/usr/local/Ascend/add-ons:/usr/local/Ascend/driver/lib64:/usr/local/dcmi

CMD /usr/local/bin/npu-exporter

