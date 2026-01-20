/* Copyright(C) 2021-2023. Huawei Technologies Co.,Ltd. All rights reserved.
   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package dcmi this for dcmi manager
package dcmi

// #cgo LDFLAGS: -ldl
/*
   #include <stddef.h>
   #include <dlfcn.h>
   #include <stdlib.h>
   #include <stdio.h>

   #include "dcmi_interface_api.h"

   static void *dcmiHandle;
   #define SO_NOT_FOUND  -99999
   #define FUNCTION_NOT_FOUND  -99998
   #define SUCCESS  0
   #define ERROR_UNKNOWN  -99997
   #define	CALL_FUNC(name,...) if(name##_func==NULL){return FUNCTION_NOT_FOUND;}return name##_func(__VA_ARGS__);

   // dcmi
   static int (*dcmi_init_func)();
   static int dcmi_init_new(){
   	CALL_FUNC(dcmi_init)
   }

   static int (*dcmi_get_card_num_list_func)(int *card_num,int *card_list,int list_length);
   static int dcmi_get_card_num_list_new(int *card_num,int *card_list,int list_length){
   	CALL_FUNC(dcmi_get_card_num_list,card_num,card_list,list_length)
   }

   static int (*dcmi_get_device_num_in_card_func)(int card_id,int *device_num);
   static int dcmi_get_device_num_in_card_new(int card_id,int *device_num){
   	CALL_FUNC(dcmi_get_device_num_in_card,card_id,device_num)
   }

   static int (*dcmi_get_device_logic_id_func)(int *device_logic_id,int card_id,int device_id);
   static int dcmi_get_device_logic_id_new(int *device_logic_id,int card_id,int device_id){
   	CALL_FUNC(dcmi_get_device_logic_id,device_logic_id,card_id,device_id)
   }

   static int (*dcmi_create_vdevice_func)(int card_id, int device_id, struct dcmi_create_vdev_res_stru *vdev,
   	struct dcmi_create_vdev_out *out);
   int dcmi_create_vdevice(int card_id, int device_id, struct dcmi_create_vdev_res_stru *vdev,
   	struct dcmi_create_vdev_out *out){
   	CALL_FUNC(dcmi_create_vdevice,card_id,device_id,vdev,out)
   }

   static int (*dcmi_get_device_info_func)(int card_id, int device_id, enum dcmi_main_cmd main_cmd,
	unsigned int sub_cmd,void *buf, unsigned int *size);
   int dcmi_get_device_info(int card_id, int device_id, enum dcmi_main_cmd main_cmd, unsigned int sub_cmd, void *buf,
   	unsigned int *size){
   	CALL_FUNC(dcmi_get_device_info,card_id,device_id,main_cmd,sub_cmd,buf,size)
   }

   static int (*dcmi_get_hccs_link_bandwidth_info_func)(int card_id, int device_id,
struct dcmi_hccs_bandwidth_info *hccs_bandwidth_info);
   int dcmi_get_hccs_link_bandwidth_info(int card_id, int device_id,
struct dcmi_hccs_bandwidth_info *hccs_bandwidth_info){
   	CALL_FUNC(dcmi_get_hccs_link_bandwidth_info,card_id,device_id,hccs_bandwidth_info)
   }

   static int (*dcmi_set_destroy_vdevice_func)(int card_id,int device_id, unsigned int VDevid);
   int dcmi_set_destroy_vdevice(int card_id,int device_id, unsigned int VDevid){
   	CALL_FUNC(dcmi_set_destroy_vdevice,card_id,device_id,VDevid)
   }

   static int (*dcmi_get_device_type_func)(int card_id,int device_id,enum dcmi_unit_type *device_type);
   int dcmi_get_device_type(int card_id,int device_id,enum dcmi_unit_type *device_type){
   	CALL_FUNC(dcmi_get_device_type,card_id,device_id,device_type)
   }

   static int (*dcmi_get_device_health_func)(int card_id, int device_id, unsigned int *health);
   int dcmi_get_device_health(int card_id, int device_id, unsigned int *health){
   	CALL_FUNC(dcmi_get_device_health,card_id,device_id,health)
   }

   static int (*dcmi_get_device_utilization_rate_func)(int card_id, int device_id, int input_type,
    unsigned int *utilization_rate);
   int dcmi_get_device_utilization_rate(int card_id, int device_id, int input_type, unsigned int *utilization_rate){
   	CALL_FUNC(dcmi_get_device_utilization_rate,card_id,device_id,input_type,utilization_rate)
   }

   static int (*dcmi_get_device_temperature_func)(int card_id, int device_id, int *temperature);
   int dcmi_get_device_temperature(int card_id, int device_id, int *temperature){
    CALL_FUNC(dcmi_get_device_temperature,card_id,device_id,temperature)
   }

   static int (*dcmi_get_device_voltage_func)(int card_id, int device_id, unsigned int *voltage);
   int dcmi_get_device_voltage(int card_id, int device_id, unsigned int *voltage){
    CALL_FUNC(dcmi_get_device_voltage,card_id,device_id,voltage)
   }

   static int (*dcmi_get_device_power_info_func)(int card_id, int device_id, int *power);
   int dcmi_get_device_power_info(int card_id, int device_id, int *power){
    CALL_FUNC(dcmi_get_device_power_info,card_id,device_id,power)
   }

   static int (*dcmi_get_device_frequency_func)(int card_id, int device_id, enum dcmi_freq_type input_type,
    unsigned int *frequency);
   int dcmi_get_device_frequency(int card_id, int device_id, enum dcmi_freq_type input_type, unsigned int *frequency){
    CALL_FUNC(dcmi_get_device_frequency,card_id,device_id,input_type,frequency)
   }

   static int (*dcmi_get_device_memory_info_v3_func)(int card_id, int device_id,
    struct dcmi_get_memory_info_stru *memory_info);
   int dcmi_get_device_memory_info_v3(int card_id, int device_id, struct dcmi_get_memory_info_stru *memory_info){
    CALL_FUNC(dcmi_get_device_memory_info_v3,card_id,device_id,memory_info)
   }

   static int (*dcmi_get_device_hbm_info_func)(int card_id, int device_id, struct dcmi_hbm_info *hbm_info);
   int dcmi_get_device_hbm_info(int card_id, int device_id, struct dcmi_hbm_info *hbm_info){
    CALL_FUNC(dcmi_get_device_hbm_info,card_id,device_id,hbm_info)
   }

   static int (*dcmi_get_device_errorcode_v2_func)(int card_id, int device_id, int *error_count,
	unsigned int *error_code_list, unsigned int list_len);
   int dcmi_get_device_errorcode_v2(int card_id, int device_id, int *error_count,
    unsigned int *error_code_list, unsigned int list_len){
    CALL_FUNC(dcmi_get_device_errorcode_v2,card_id,device_id,error_count,error_code_list,list_len)
   }

   static int (*dcmi_get_device_chip_info_func)(int card_id, int device_id, struct dcmi_chip_info *chip_info);
   int dcmi_get_device_chip_info(int card_id, int device_id, struct dcmi_chip_info *chip_info){
    CALL_FUNC(dcmi_get_device_chip_info,card_id,device_id,chip_info)
   }

   static int (*dcmi_get_device_chip_info_v2_func)(int card_id, int device_id, struct dcmi_chip_info_v2 *chip_info);
   int dcmi_get_device_chip_info_v2(int card_id, int device_id, struct dcmi_chip_info_v2 *chip_info){
    CALL_FUNC(dcmi_get_device_chip_info_v2,card_id,device_id,chip_info)
   }

   static int (*dcmi_get_device_phyid_from_logicid_func)(unsigned int logicid, unsigned int *phyid);
   int dcmi_get_device_phyid_from_logicid(unsigned int logicid, unsigned int *phyid){
    CALL_FUNC(dcmi_get_device_phyid_from_logicid,logicid,phyid)
   }

   static int (*dcmi_get_device_logicid_from_phyid_func)(unsigned int phyid, unsigned int *logicid);
   int dcmi_get_device_logicid_from_phyid(unsigned int phyid, unsigned int *logicid){
    CALL_FUNC(dcmi_get_device_logicid_from_phyid,phyid,logicid)
   }

   static int (*dcmi_get_device_ip_func)(int card_id, int device_id, enum dcmi_port_type input_type, int port_id,
    struct dcmi_ip_addr *ip, struct dcmi_ip_addr *mask);
   int dcmi_get_device_ip(int card_id, int device_id, enum dcmi_port_type input_type, int port_id,
    struct dcmi_ip_addr *ip, struct dcmi_ip_addr *mask){
    CALL_FUNC(dcmi_get_device_ip,card_id,device_id,input_type,port_id,ip,mask)
   }

   static int (*dcmi_get_device_network_health_func)(int card_id, int device_id,
	enum dcmi_rdfx_detect_result *result);
   int dcmi_get_device_network_health(int card_id, int device_id, enum dcmi_rdfx_detect_result *result){
    CALL_FUNC(dcmi_get_device_network_health,card_id,device_id,result)
   }

   static int (*dcmi_get_card_list_func)(int *card_num, int *card_list, int list_len);
   int dcmi_get_card_list(int *card_num, int *card_list, int list_len){
    CALL_FUNC(dcmi_get_card_list,card_num,card_list,list_len)
   }

   static int (*dcmi_get_device_id_in_card_func)(int card_id, int *device_id_max, int *mcu_id, int *cpu_id);
   int dcmi_get_device_id_in_card(int card_id, int *device_id_max, int *mcu_id, int *cpu_id){
    CALL_FUNC(dcmi_get_device_id_in_card,card_id,device_id_max,mcu_id,cpu_id)
   }

   static int (*dcmi_get_memory_info_func)(int card_id, int device_id,
	struct dcmi_memory_info_stru *device_memory_info);
   int dcmi_get_memory_info(int card_id, int device_id, struct dcmi_memory_info_stru *device_memory_info){
    CALL_FUNC(dcmi_get_memory_info,card_id,device_id,device_memory_info)
   }

   static int (*dcmi_get_device_errorcode_func)(int card_id, int device_id, int *error_count, unsigned int *error_code,
   int *error_width);
   int dcmi_get_device_errorcode(int card_id, int device_id, int *error_count, unsigned int *error_code,
   int *error_width){
    CALL_FUNC(dcmi_get_device_errorcode,card_id,device_id,error_count,error_code,error_width)
   }

   static int (*dcmi_get_card_id_device_id_from_logicid_func)(int *card_id, int *device_id,
	unsigned int device_logic_id);
   int dcmi_get_card_id_device_id_from_logicid(int *card_id, int *device_id, unsigned int device_logic_id){
    CALL_FUNC(dcmi_get_card_id_device_id_from_logicid,card_id,device_id,device_logic_id)
   }

   static int (*dcmi_mcu_get_power_info_func)(int card_id, int *power);
   static int dcmi_mcu_get_power_info_new(int card_id, int *power){
    CALL_FUNC(dcmi_mcu_get_power_info,card_id,power)
   }

   static int (*dcmi_get_product_type_func)(int card_id, int device_id, char *product_type_str, int buf_size);
   int dcmi_get_product_type(int card_id, int device_id, char *product_type_str, int buf_size){
    CALL_FUNC(dcmi_get_product_type,card_id,device_id,product_type_str,buf_size)
   }

   static int (*dcmi_get_card_elabel_v2_func)(int card_id, struct dcmi_elabel_info *elabel_info);
   int dcmi_get_card_elabel_v2(int card_id, struct dcmi_elabel_info *elabel_info){
    CALL_FUNC(dcmi_get_card_elabel_v2,card_id,elabel_info)
   }

   static int (*dcmi_set_device_reset_func)(int card_id, int device_id, enum dcmi_reset_channel channel_type);
   int dcmi_set_device_reset(int card_id, int device_id, enum dcmi_reset_channel channel_type){
    CALL_FUNC(dcmi_set_device_reset,card_id,device_id,channel_type)
   }

	static int (*dcmi_get_device_outband_channel_state_func)(int card_id, int device_id, int* channel_state);
	int dcmi_get_device_outband_channel_state(int card_id, int device_id, int* channel_state){
	 CALL_FUNC(dcmi_get_device_outband_channel_state,card_id,device_id,channel_state)
	}

	static int (*dcmi_pre_reset_soc_func)(int card_id, int device_id);
	int dcmi_pre_reset_soc(int card_id, int device_id){
	 CALL_FUNC(dcmi_pre_reset_soc,card_id,device_id)
	}

	static int (*dcmi_rescan_soc_func)(int card_id, int device_id);
	int dcmi_rescan_soc(int card_id, int device_id){
	 CALL_FUNC(dcmi_rescan_soc,card_id,device_id)
	}

	static int (*dcmi_get_netdev_brother_device_func)(int card_id, int device_id, int* brother_card_id);
	int dcmi_get_netdev_brother_device(int card_id, int device_id, int* brother_card_id){
	 CALL_FUNC(dcmi_get_netdev_brother_device,card_id,device_id,brother_card_id)
	}

   static int (*dcmi_get_device_boot_status_func)(int card_id, int device_id, enum dcmi_boot_status *boot_status);
   int dcmi_get_device_boot_status(int card_id, int device_id, enum dcmi_boot_status *boot_status){
    CALL_FUNC(dcmi_get_device_boot_status,card_id,device_id,boot_status)
   }

    void goEventFaultCallBack(struct dcmi_dms_fault_event);
    static void event_handler(struct dcmi_event *fault_event) {
        goEventFaultCallBack(fault_event->event_t.dms_event);
    }

    static int (*dcmi_subscribe_fault_event_func)(int card_id, int device_id, struct dcmi_event_filter filter,
        void (*f_name)(struct dcmi_event *fault_event));
       int dcmi_subscribe_fault_event(int card_id, int device_id, struct dcmi_event_filter filter){
        CALL_FUNC(dcmi_subscribe_fault_event,card_id,device_id,filter,event_handler)
       }

    static int (*dcmi_get_npu_work_mode_func)(int card_id, unsigned char *work_mode);
    int dcmi_get_npu_work_mode(int card_id, unsigned char *work_mode){
        CALL_FUNC(dcmi_get_npu_work_mode,card_id,work_mode)
    }

    static int (*dcmi_get_device_die_v2_func)(int card_id, int device_id, enum dcmi_die_type input_type,
    struct dcmi_die_id *die_id);
    int dcmi_get_device_die_v2(int card_id, int device_id, enum dcmi_die_type input_type, struct dcmi_die_id *die_id){
        CALL_FUNC(dcmi_get_device_die_v2,card_id,device_id,input_type,die_id)
    }

    static int (*dcmi_get_device_resource_info_func)(int card_id, int device_id, struct dcmi_proc_mem_info *proc_info,
    int *proc_num);
    int dcmi_get_device_resource_info(int card_id, int device_id, struct dcmi_proc_mem_info *proc_info, int *proc_num){
        CALL_FUNC(dcmi_get_device_resource_info,card_id,device_id,proc_info,proc_num)
    }

    static int (*dcmi_get_device_pcie_info_v2_func)(int card_id, int device_id, struct dcmi_pcie_info_all *pcie_info);
    int dcmi_get_device_pcie_info_v2(int card_id, int device_id, struct dcmi_pcie_info_all *pcie_info){
        CALL_FUNC(dcmi_get_device_pcie_info_v2,card_id,device_id,pcie_info)
    }

    static int (*dcmi_get_device_board_info_func)(int card_id, int device_id, struct dcmi_board_info *board_info);
    int dcmi_get_device_board_info(int card_id, int device_id, struct dcmi_board_info *board_info){
        CALL_FUNC(dcmi_get_device_board_info,card_id,device_id,board_info)
    }

    static int (*dcmi_get_pcie_link_bandwidth_info_func)(int card_id, int device_id,
    struct dcmi_pcie_link_bandwidth_info *pcie_link_bandwidth_info);
    int dcmi_get_pcie_link_bandwidth_info(int card_id, int device_id,
    struct dcmi_pcie_link_bandwidth_info *pcie_link_bandwidth_info){
        CALL_FUNC(dcmi_get_pcie_link_bandwidth_info,card_id,device_id,pcie_link_bandwidth_info)
    }

   static int (*dcmi_get_dcmi_version_func)(char *dcmi_ver, int buf_size);
   int dcmi_get_dcmi_version(char *dcmi_ver, int buf_size){
    CALL_FUNC(dcmi_get_dcmi_version,dcmi_ver,buf_size)
   }

   static int (*dcmi_get_device_ecc_info_func)(int card_id, int device_id, enum dcmi_device_type input_type,
    struct dcmi_ecc_info *device_ecc_info);
   int dcmi_get_device_ecc_info(int card_id, int device_id, enum dcmi_device_type input_type,
    struct dcmi_ecc_info *device_ecc_info){
    CALL_FUNC(dcmi_get_device_ecc_info,card_id,device_id,input_type,device_ecc_info)
   }

    static int (*dcmi_get_mainboard_id_func)(int card_id, int device_id, unsigned int *mainboard_id);
    int dcmi_get_mainboard_id(int card_id, int device_id, unsigned int *mainboard_id){
        CALL_FUNC(dcmi_get_mainboard_id,card_id,device_id,mainboard_id)
    }

	static int (*dcmi_start_hccsping_mesh_func)(int card_id, int device_id, int port_id,
struct dcmi_hccsping_mesh_operate *hccsping_mesh);
    int dcmi_start_hccsping_mesh(int card_id, int device_id, int port_id,
struct dcmi_hccsping_mesh_operate *hccsping_mesh){
		CALL_FUNC(dcmi_start_hccsping_mesh,card_id,device_id,port_id,hccsping_mesh)
}
	static int (*dcmi_stop_hccsping_mesh_func)(int card_id, int device_id, int port_id, unsigned int task_id);
	int dcmi_stop_hccsping_mesh(int card_id, int device_id, int port_id, unsigned int task_id){
		CALL_FUNC(dcmi_stop_hccsping_mesh,card_id,device_id,port_id,task_id)
	}

	static int (*dcmi_get_hccsping_mesh_info_func)(int card_id, int device_id, int port_id, unsigned int task_id,
struct dcmi_hccsping_mesh_info *hccsping_mesh_info);
	int dcmi_get_hccsping_mesh_info(int card_id, int device_id, int port_id, unsigned int task_id,
struct dcmi_hccsping_mesh_info *hccsping_mesh_info){
        CALL_FUNC(dcmi_get_hccsping_mesh_info,card_id,device_id,port_id,task_id,hccsping_mesh_info)
}

	static int (*dcmi_get_hccsping_mesh_state_func)(int card_id, int device_id, int port_id, unsigned int task_id,
unsigned int *state);
	int dcmi_get_hccsping_mesh_state(int card_id, int device_id, int port_id, unsigned int task_id,
unsigned int *state){
		CALL_FUNC(dcmi_get_hccsping_mesh_state,card_id,device_id,port_id,task_id,state)
}

	static int (*dcmi_get_spod_node_status_func)(int card_id, int device_id, unsigned int sdid, unsigned int *status);
	int dcmi_get_spod_node_status(int card_id, int device_id, unsigned int sdid, unsigned int *status){
		CALL_FUNC(dcmi_get_spod_node_status,card_id,device_id,sdid,status)
	}

	static int (*dcmi_set_spod_node_status_func)(int card_id, int device_id, unsigned int sdid, unsigned int status);
	int dcmi_set_spod_node_status(int card_id, int device_id, unsigned int sdid, unsigned int status){
		CALL_FUNC(dcmi_set_spod_node_status,card_id,device_id,sdid,status)
	}

   // load .so files and functions
   static int dcmiInit_dl(const char* dcmiLibPath){
   	if (dcmiLibPath == NULL) {
   	   	fprintf (stderr,"lib path is null\n");
   	   	return SO_NOT_FOUND;
   	}
   	dcmiHandle = dlopen(dcmiLibPath,RTLD_LAZY | RTLD_GLOBAL);
   	if (dcmiHandle == NULL){
   		fprintf (stderr,"%s\n",dlerror());
   		return SO_NOT_FOUND;
   	}

   	dcmi_init_func = dlsym(dcmiHandle,"dcmi_init");

   	dcmi_get_card_num_list_func = dlsym(dcmiHandle,"dcmi_get_card_num_list");

   	dcmi_get_device_num_in_card_func = dlsym(dcmiHandle,"dcmi_get_device_num_in_card");

   	dcmi_get_device_logic_id_func = dlsym(dcmiHandle,"dcmi_get_device_logic_id");

   	dcmi_create_vdevice_func = dlsym(dcmiHandle,"dcmi_create_vdevice");

   	dcmi_get_device_info_func = dlsym(dcmiHandle,"dcmi_get_device_info");

   	dcmi_set_destroy_vdevice_func = dlsym(dcmiHandle,"dcmi_set_destroy_vdevice");

   	dcmi_get_device_type_func = dlsym(dcmiHandle,"dcmi_get_device_type");

   	dcmi_get_device_health_func = dlsym(dcmiHandle,"dcmi_get_device_health");

   	dcmi_get_device_utilization_rate_func = dlsym(dcmiHandle,"dcmi_get_device_utilization_rate");

   	dcmi_get_device_temperature_func = dlsym(dcmiHandle,"dcmi_get_device_temperature");

   	dcmi_get_device_voltage_func = dlsym(dcmiHandle,"dcmi_get_device_voltage");

   	dcmi_get_device_power_info_func = dlsym(dcmiHandle,"dcmi_get_device_power_info");

   	dcmi_get_device_frequency_func = dlsym(dcmiHandle,"dcmi_get_device_frequency");

   	dcmi_get_device_memory_info_v3_func = dlsym(dcmiHandle,"dcmi_get_device_memory_info_v3");

   	dcmi_get_device_hbm_info_func = dlsym(dcmiHandle,"dcmi_get_device_hbm_info");

   	dcmi_get_device_errorcode_v2_func = dlsym(dcmiHandle,"dcmi_get_device_errorcode_v2");

   	dcmi_get_device_chip_info_func = dlsym(dcmiHandle,"dcmi_get_device_chip_info");

   	dcmi_get_device_chip_info_v2_func = dlsym(dcmiHandle,"dcmi_get_device_chip_info_v2");

   	dcmi_get_device_phyid_from_logicid_func = dlsym(dcmiHandle,"dcmi_get_device_phyid_from_logicid");

   	dcmi_get_device_logicid_from_phyid_func = dlsym(dcmiHandle,"dcmi_get_device_logicid_from_phyid");

   	dcmi_get_device_ip_func = dlsym(dcmiHandle,"dcmi_get_device_ip");

   	dcmi_get_device_network_health_func = dlsym(dcmiHandle,"dcmi_get_device_network_health");

   	dcmi_get_card_list_func = dlsym(dcmiHandle,"dcmi_get_card_list");

   	dcmi_get_device_id_in_card_func = dlsym(dcmiHandle,"dcmi_get_device_id_in_card");

   	dcmi_get_memory_info_func = dlsym(dcmiHandle,"dcmi_get_memory_info");

   	dcmi_get_device_errorcode_func = dlsym(dcmiHandle,"dcmi_get_device_errorcode");

   	dcmi_get_card_id_device_id_from_logicid_func = dlsym(dcmiHandle,"dcmi_get_card_id_device_id_from_logicid");

   	dcmi_mcu_get_power_info_func = dlsym(dcmiHandle,"dcmi_mcu_get_power_info");

   	dcmi_get_product_type_func = dlsym(dcmiHandle,"dcmi_get_product_type");

   	dcmi_get_card_elabel_v2_func = dlsym(dcmiHandle,"dcmi_get_card_elabel_v2");

   	dcmi_set_device_reset_func = dlsym(dcmiHandle,"dcmi_set_device_reset");

	dcmi_get_device_outband_channel_state_func = dlsym(dcmiHandle,"dcmi_get_device_outband_channel_state");

	dcmi_pre_reset_soc_func = dlsym(dcmiHandle,"dcmi_pre_reset_soc");

	dcmi_rescan_soc_func = dlsym(dcmiHandle,"dcmi_rescan_soc");

	dcmi_get_netdev_brother_device_func = dlsym(dcmiHandle,"dcmi_get_netdev_brother_device");

   	dcmi_get_device_boot_status_func = dlsym(dcmiHandle,"dcmi_get_device_boot_status");

   	dcmi_subscribe_fault_event_func = dlsym(dcmiHandle,"dcmi_subscribe_fault_event");

   	dcmi_get_npu_work_mode_func = dlsym(dcmiHandle, "dcmi_get_npu_work_mode");

    dcmi_get_device_die_v2_func = dlsym(dcmiHandle, "dcmi_get_device_die_v2");

    dcmi_get_device_resource_info_func = dlsym(dcmiHandle, "dcmi_get_device_resource_info");

    dcmi_get_device_pcie_info_v2_func = dlsym(dcmiHandle, "dcmi_get_device_pcie_info_v2");

    dcmi_get_device_board_info_func = dlsym(dcmiHandle, "dcmi_get_device_board_info");

    dcmi_get_pcie_link_bandwidth_info_func = dlsym(dcmiHandle, "dcmi_get_pcie_link_bandwidth_info");

    dcmi_get_dcmi_version_func = dlsym(dcmiHandle,"dcmi_get_dcmi_version");

	dcmi_get_device_ecc_info_func = dlsym(dcmiHandle,"dcmi_get_device_ecc_info");

    dcmi_get_mainboard_id_func = dlsym(dcmiHandle, "dcmi_get_mainboard_id");

   	dcmi_get_hccs_link_bandwidth_info_func = dlsym(dcmiHandle,"dcmi_get_hccs_link_bandwidth_info");

	dcmi_start_hccsping_mesh_func = dlsym(dcmiHandle,"dcmi_start_hccsping_mesh");

	dcmi_stop_hccsping_mesh_func = dlsym(dcmiHandle,"dcmi_stop_hccsping_mesh");

	dcmi_get_hccsping_mesh_info_func = dlsym(dcmiHandle,"dcmi_get_hccsping_mesh_info");

	dcmi_get_hccsping_mesh_state_func = dlsym(dcmiHandle,"dcmi_get_hccsping_mesh_state");

	dcmi_get_spod_node_status_func = dlsym(dcmiHandle,"dcmi_get_spod_node_status");

	dcmi_set_spod_node_status_func = dlsym(dcmiHandle,"dcmi_set_spod_node_status");

   	return SUCCESS;
   }

   static int dcmiShutDown(void){
   	if (dcmiHandle == NULL) {
   		return SUCCESS;
   	}
   	return (dlclose(dcmiHandle) ? ERROR_UNKNOWN : SUCCESS);
   }
*/
import "C"
import (
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"ascend-common/common-utils/hwlog"
	"ascend-common/common-utils/utils"
	"ascend-common/devmanager/common"
)

// CDcmiMemoryInfoV3 the c struct of memoryInfo for v3
type CDcmiMemoryInfoV3 = C.struct_dcmi_get_memory_info_stru

// CDcmiMemoryInfoV1 the c struct of memoryInfo for v1
type CDcmiMemoryInfoV1 = C.struct_dcmi_memory_info_stru

// DcDriverInterface interface for dcmi
type DcDriverInterface interface {
	DcInit() error
	DcShutDown() error

	DcGetDcmiVersion() (string, error)
	DcGetDeviceCount() (int32, error)
	DcGetLogicIDList() (int32, []int32, error)
	DcGetDeviceHealth(int32, int32) (int32, error)
	DcGetDeviceNetWorkHealth(int32, int32) (uint32, error)
	DcGetDeviceUtilizationRate(int32, int32, common.DeviceType) (int32, error)
	DcGetDeviceTemperature(int32, int32) (int32, error)
	DcGetDeviceVoltage(int32, int32) (float32, error)
	DcGetDevicePowerInfo(int32, int32) (float32, error)
	DcGetDeviceFrequency(int32, int32, common.DeviceType) (uint32, error)
	DcGetMemoryInfo(int32, int32) (*common.MemoryInfo, error)
	DcGetHbmInfo(int32, int32) (*common.HbmInfo, error)
	DcGetDeviceErrorCode(int32, int32) (int32, int64, error)
	DcGetChipInfo(int32, int32) (*common.ChipInfo, error)
	DcGetPhysicIDFromLogicID(int32) (int32, error)
	DcGetLogicIDFromPhysicID(int32) (int32, error)
	DcGetDeviceLogicID(int32, int32) (int32, error)
	DcGetDeviceIPAddress(int32, int32, int32) (string, error)
	DcGetMcuPowerInfo(int32) (float32, error)
	DcGetDieID(int32, int32, DieType) (string, error)
	DcGetPCIeBusInfo(int32, int32) (string, error)

	DcGetCardList() (int32, []int32, error)
	DcGetDeviceNumInCard(int32) (int32, error)
	DcSetDestroyVirtualDevice(int32, int32, uint32) error
	DcCreateVirtualDevice(int32, int32, common.CgoCreateVDevRes) (common.CgoCreateVDevOut, error)
	DcGetDeviceVDevResource(int32, int32, uint32) (common.CgoVDevQueryStru, error)
	DcGetDeviceTotalResource(int32, int32) (common.CgoSocTotalResource, error)
	DcGetDeviceFreeResource(int32, int32) (common.CgoSocFreeResource, error)
	DcGetVDevActivityInfo(int32, int32, uint32) (common.VDevActivityInfo, error)
	DcVGetDeviceInfo(int32, int32) (common.VirtualDevInfo, error)
	DcGetCardIDDeviceID(int32) (int32, int32, error)
	DcCreateVDevice(int32, common.CgoCreateVDevRes) (common.CgoCreateVDevOut, error)
	DcGetVDeviceInfo(int32) (common.VirtualDevInfo, error)
	DcDestroyVDevice(int32, uint32) error
	DcGetProductType(int32, int32) (string, error)
	DcGetNpuWorkMode(int32) (int, error)
	DcSetDeviceReset(int32, int32) error
	DcGetBrotherCardID(int32, int32) (int32, error)
	DcPreResetSoc(int32, int32) error
	DcGetOutBandChannelState(int32, int32) error
	DcSetDeviceResetOutBand(int32, int32) error
	DcRescanSoc(int32, int32) error
	DcGetDeviceBootStatus(int32) (int, error)
	DcGetSuperPodInfo(int32, int32) (common.CgoSuperPodInfo, error)

	DcGetDeviceAllErrorCode(int32, int32) (int32, []int64, error)
	DcSubscribeDeviceFaultEvent(int32, int32) error
	DcSetFaultEventCallFunc(func(common.DevFaultInfo))
	DcGetDevProcessInfo(int32, int32) (*common.DevProcessInfo, error)
	DcGetDeviceBoardInfo(int32, int32) (common.BoardInfo, error)
	DcGetPCIEBandwidth(int32, int32, int) (common.PCIEBwStat, error)
	DcGetDeviceEccInfo(int32, int32, common.DcmiDeviceType) (*common.ECCInfo, error)
	DcGetSioInfo(int32, int32) (common.SioCrcErrStatisticInfo, error)
	DcGetHccsStatisticInfo(int32, int32) (common.HccsStatisticInfo, error)
	DcGetHccsStatisticInfoU64(int32, int32) (common.HccsStatisticInfo, error)
	DcGetDeviceMainBoardInfo(int32, int32) (uint32, error)
	DcGetHccsBandwidthInfo(int32, int32, int) (common.HccsBandwidthInfo, error)

	DcStartHccsPingMesh(int32, int32, int, common.HccspingMeshOperate) error
	DcStopHccsPingMesh(int32, int32, int, uint) error
	DcGetHccsPingMeshInfo(int32, int32, int, uint) (*common.HccspingMeshInfo, error)
	DcGetHccsPingMeshState(int32, int32, int, uint) (int, error)
	DcGetSuperPodStatus(int32, int32, uint32) (int, error)
	DcSetSuperPodStatus(int32, int32, uint32, uint32) error
	DcGetCardElabelV2(int32) (common.ElabelInfo, error)
}

const (
	dcmiLibraryName    = "libdcmi.so"
	templateNameLen    = 32
	ipAddrListLen      = 1024
	hcclpingMeshMaxNum = 48
)

var faultEventCallFunc func(common.DevFaultInfo) = nil
var (
	dcmiErrMap = map[int32]string{
		-8001:  "The input parameter is incorrect",
		-8002:  "Permission error",
		-8003:  "The memory interface operation failed",
		-8004:  "The security function failed to be executed",
		-8005:  "Internal errors",
		-8006:  "Response timed out",
		-8007:  "Invalid deviceID",
		-8008:  "The device does not exist",
		-8009:  "ioctl returns failed",
		-8010:  "The message failed to be sent",
		-8011:  "Message reception failed",
		-8012:  "Not ready yet,please try again",
		-8013:  "This API is not supported in containers",
		-8014:  "The file operation failed",
		-8015:  "Reset failed",
		-8016:  "Reset cancels",
		-8017:  "Upgrading",
		-8020:  "Device resources are occupied",
		-8022:  "Partition consistency check,inconsistent partitions were found",
		-8023:  "The configuration information does not exist",
		-8255:  "Device ID/function is not supported",
		-99997: "dcmi shutdown failed",
		-99998: "The called function is missing,please upgrade the driver",
		-99999: "dcmi libdcmi.so failed to load",
	}
)

// DcManager for manager dcmi interface
type DcManager struct{}

// DcStartHccsPingMesh start hccs ping mesh
func (d *DcManager) DcStartHccsPingMesh(cardID int32, deviceID int32, portID int,
	operate common.HccspingMeshOperate) error {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	if !common.IsValidPortID(portID) {
		return fmt.Errorf("portID(%d) is invalid", portID)
	}
	if err := common.IsValidHccspingMeshOperate(operate); err != nil {
		return fmt.Errorf("operate(%v) is invalid, err: %v", operate, err)
	}
	dtsAddrLsit := [ipAddrListLen]C.char{0}
	for i := 0; i < len(operate.DstAddr) && i < len(dtsAddrLsit); i++ {
		dtsAddrLsit[i] = C.char(operate.DstAddr[i])
	}

	op := C.struct_dcmi_hccsping_mesh_operate{
		dst_addr_list: dtsAddrLsit,
		pkt_size:      C.int(operate.PktSize),
		pkt_send_num:  C.int(operate.PktSendNum),
		pkt_interval:  C.int(operate.PktInterval),
		timeout:       C.int(operate.Timeout),
		task_interval: C.int(operate.TaskInterval),
		task_id:       C.int(operate.TaskId),
	}
	if retCode := C.dcmi_start_hccsping_mesh(C.int(cardID), C.int(deviceID), C.int(portID),
		&op); retCode != common.Success {
		return fmt.Errorf("dcmi start hccs ping mesh failed cardID(%d) deviceID(%d) error code: %d",
			cardID, deviceID, int32(retCode))
	}

	return nil
}

// DcStopHccsPingMesh stop hccs ping mesh
func (d *DcManager) DcStopHccsPingMesh(cardID int32, deviceID int32, portID int, taskID uint) error {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	if !common.IsValidPortID(portID) {
		return fmt.Errorf("portID(%d) is invalid", portID)
	}
	if !common.IsValidTaskID(taskID) {
		return fmt.Errorf("taskID(%d) is invalid", taskID)
	}
	if retCode := C.dcmi_stop_hccsping_mesh(C.int(cardID), C.int(deviceID), C.int(portID),
		C.uint(taskID)); retCode != common.Success {
		return fmt.Errorf("dcmi stop hccs ping mesh failed cardID(%d) deviceID(%d) error code: %d",
			cardID, deviceID, int32(retCode))
	}
	return nil
}

// DcGetHccsPingMeshInfo get hccs ping mesh info
func (d *DcManager) DcGetHccsPingMeshInfo(cardID int32, deviceID int32, portID int,
	taskID uint) (*common.HccspingMeshInfo, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return nil, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	if !common.IsValidPortID(portID) {
		return nil, fmt.Errorf("portID(%d) is invalid", portID)
	}
	if !common.IsValidTaskID(taskID) {
		return nil, fmt.Errorf("taskID(%d) is invalid", taskID)
	}
	var info C.struct_dcmi_hccsping_mesh_info
	if retCode := C.dcmi_get_hccsping_mesh_info(C.int(cardID), C.int(deviceID), C.int(portID), C.uint(taskID),
		&info); retCode != common.Success {
		return nil, fmt.Errorf("dcmi get hccs ping mesh info failed cardID(%d) deviceID(%d) error code: %d",
			cardID, deviceID, int32(retCode))
	}
	return convertHccspingMeshInfo(&info)
}

func convertHccspingMeshInfo(cInfo *C.struct_dcmi_hccsping_mesh_info) (*common.HccspingMeshInfo, error) {
	if int(cInfo.dest_num) > hcclpingMeshMaxNum {
		return nil, fmt.Errorf("dest_num(%d) is invalid, should not be greater than %d", int(cInfo.dest_num),
			hcclpingMeshMaxNum)
	}
	info := &common.HccspingMeshInfo{}
	for i := 0; i < int(cInfo.dest_num); i++ {
		info.DstAddr = append(info.DstAddr, convertToString(cInfo.dst_addr[i]))
		info.SucPktNum = append(info.SucPktNum, uint(cInfo.suc_pkt_num[i]))
		info.FailPktNum = append(info.FailPktNum, uint(cInfo.fail_pkt_num[i]))
		info.MaxTime = append(info.MaxTime, int(cInfo.max_time[i]))
		info.MinTime = append(info.MinTime, int(cInfo.min_time[i]))
		info.AvgTime = append(info.AvgTime, int(cInfo.avg_time[i]))
		info.TP95Time = append(info.TP95Time, int(cInfo.tp95_time[i]))
		info.ReplyStatNum = append(info.ReplyStatNum, int(cInfo.reply_stat_num[i]))
		info.PingTotalNum = append(info.PingTotalNum, int(cInfo.ping_total_num[i]))
	}
	info.DestNum = int(cInfo.dest_num)
	return info, nil
}

// DcGetHccsPingMeshState get hccs ping mesh state
func (d *DcManager) DcGetHccsPingMeshState(cardID int32, deviceID int32, portID int, taskID uint) (int, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.RetError, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	if !common.IsValidPortID(portID) {
		return common.RetError, fmt.Errorf("portID(%d) is invalid", portID)
	}
	if !common.IsValidTaskID(taskID) {
		return common.RetError, fmt.Errorf("taskID(%d) is invalid", taskID)
	}
	var state C.uint
	if retCode := C.dcmi_get_hccsping_mesh_state(C.int(cardID), C.int(deviceID), C.int(portID), C.uint(taskID),
		&state); retCode != common.Success {
		return common.RetError, fmt.Errorf("dcmi get hccs ping mesh state failed cardID(%d) deviceID(%d) error "+
			"code: %d", cardID, deviceID, int32(retCode))
	}
	return int(state), nil
}

// DcInit load symbol and initialize dcmi
func (d *DcManager) DcInit() error {
	dcmiLibPath, err := utils.GetDriverLibPath(dcmiLibraryName)
	if err != nil {
		return err
	}
	cDcmiTemplateName := C.CString(dcmiLibPath)
	defer C.free(unsafe.Pointer(cDcmiTemplateName))
	if retCode := C.dcmiInit_dl(cDcmiTemplateName); retCode != C.SUCCESS {
		return fmt.Errorf("dcmi lib load failed, error code: %d", int32(retCode))
	}
	if retCode := C.dcmi_init_new(); retCode != C.SUCCESS {
		return fmt.Errorf("dcmi init failed, error code: %d", int32(retCode))
	}
	return nil
}

// DcShutDown clean the dynamically loaded resource
func (d *DcManager) DcShutDown() error {
	if retCode := C.dcmiShutDown(); retCode != C.SUCCESS {
		return fmt.Errorf("dcmi shut down failed, error code: %d", int32(retCode))
	}

	return nil
}

// DcGetCardList get card list
func (d *DcManager) DcGetCardList() (int32, []int32, error) {
	var ids [common.HiAIMaxCardNum]C.int
	var cNum C.int
	if retCode := C.dcmi_get_card_list(&cNum, &ids[0], common.HiAIMaxCardNum); int32(retCode) != common.
		Success {
		return common.RetError, nil, fmt.Errorf("get card list failed, error code: %d", int32(retCode))
	}
	// checking card's quantity
	if cNum <= 0 || cNum > common.HiAIMaxCardNum {
		return common.RetError, nil, fmt.Errorf("get error card quantity: %d", int32(cNum))
	}
	var cardNum = int32(cNum)
	var i int32
	var cardIDList []int32
	for i = 0; i < cardNum; i++ {
		cardID := int32(ids[i])
		if cardID < 0 {
			hwlog.RunLog.Errorf("get invalid card ID: %d", cardID)
			continue
		}
		cardIDList = append(cardIDList, cardID)
	}
	return cardNum, cardIDList, nil
}

// DcGetDeviceNumInCard get device number in the npu card
func (d *DcManager) DcGetDeviceNumInCard(cardID int32) (int32, error) {
	if !common.IsValidCardID(cardID) {
		return common.RetError, fmt.Errorf("cardID(%d) is invalid", cardID)
	}
	var deviceNum C.int
	if retCode := C.dcmi_get_device_num_in_card_new(C.int(cardID), &deviceNum); int32(retCode) != common.Success {
		return common.RetError, fmt.Errorf("get device count on the card failed, error code: %d", int32(retCode))
	}
	if !common.IsValidDevNumInCard(int32(deviceNum)) {
		return common.RetError, fmt.Errorf("get error device quantity: %d", int32(deviceNum))
	}
	return int32(deviceNum), nil
}

// DcGetDeviceLogicID get device logicID
func (d *DcManager) DcGetDeviceLogicID(cardID, deviceID int32) (int32, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.RetError, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var logicID C.int
	if retCode := C.dcmi_get_device_logic_id_new(&logicID, C.int(cardID),
		C.int(deviceID)); int32(retCode) != common.Success {
		return common.RetError, fmt.Errorf("failed to get logicID by cardID(%d) and deviceID(%d), error code: %d",
			cardID, deviceID, int32(retCode))
	}

	// check whether logicID is invalid
	if !common.IsValidLogicIDOrPhyID(int32(logicID)) {
		return common.RetError, fmt.Errorf("get invalid logicID: %d", int32(logicID))
	}
	return int32(logicID), nil
}

// DcSetDestroyVirtualDevice destroy virtual device
func (d *DcManager) DcSetDestroyVirtualDevice(cardID, deviceID int32, vDevID uint32) error {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	if retCode := C.dcmi_set_destroy_vdevice(C.int(cardID), C.int(deviceID),
		C.uint(vDevID)); int32(retCode) != common.Success {
		return fmt.Errorf("destroy virtual device failed, error code: %d", int32(retCode))
	}
	return nil
}

func convertCreateVDevOut(cCreateVDevOut C.struct_dcmi_create_vdev_out) common.CgoCreateVDevOut {
	cgoCreateVDevOut := common.CgoCreateVDevOut{
		VDevID:     uint32(cCreateVDevOut.vdev_id),
		PcieBus:    uint32(cCreateVDevOut.pcie_bus),
		PcieDevice: uint32(cCreateVDevOut.pcie_device),
		PcieFunc:   uint32(cCreateVDevOut.pcie_func),
		VfgID:      uint32(cCreateVDevOut.vfg_id),
	}
	return cgoCreateVDevOut
}

// DcCreateVirtualDevice create virtual device
func (d *DcManager) DcCreateVirtualDevice(cardID, deviceID int32, vDevInfo common.CgoCreateVDevRes) (common.
	CgoCreateVDevOut, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.CgoCreateVDevOut{}, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	if len(vDevInfo.TemplateName) > templateNameLen {
		return common.CgoCreateVDevOut{}, fmt.Errorf("the length of template name exceeds the upper limit")
	}
	cTemplateName := [templateNameLen]C.char{0}
	for i := 0; i < len(vDevInfo.TemplateName); i++ {
		cTemplateName[i] = C.char(vDevInfo.TemplateName[i])
	}
	deviceCreateStr := C.struct_dcmi_create_vdev_res_stru{
		vdev_id:       C.uint(vDevInfo.VDevID),
		vfg_id:        C.uint(vDevInfo.VfgID),
		template_name: cTemplateName,
	}

	var createVDevOut C.struct_dcmi_create_vdev_out
	if retCode := C.dcmi_create_vdevice(C.int(cardID), C.int(deviceID), &deviceCreateStr,
		&createVDevOut); int32(retCode) != common.Success {
		return common.CgoCreateVDevOut{}, fmt.Errorf("create vdevice failed, error is: %d", int32(retCode))
	}

	return convertCreateVDevOut(createVDevOut), nil
}

func convertToString(cgoArr [dcmiVDevResNameLen]C.char) string {
	var charArr []rune
	for _, v := range cgoArr {
		if v == 0 {
			break
		}
		charArr = append(charArr, rune(v))
	}
	return string(charArr)
}

func convertBaseResource(cBaseResource C.struct_dcmi_base_resource) common.CgoBaseResource {
	baseResource := common.CgoBaseResource{
		Token:       uint64(cBaseResource.token),
		TokenMax:    uint64(cBaseResource.token_max),
		TaskTimeout: uint64(cBaseResource.task_timeout),
		VfgID:       uint32(cBaseResource.vfg_id),
		VipMode:     uint8(cBaseResource.vip_mode),
	}
	return baseResource
}

func convertComputingResource(cComputingResource C.struct_dcmi_computing_resource) common.CgoComputingResource {
	computingResource := common.CgoComputingResource{
		Aic:                float32(cComputingResource.aic),
		Aiv:                float32(cComputingResource.aiv),
		Dsa:                uint16(cComputingResource.dsa),
		Rtsq:               uint16(cComputingResource.rtsq),
		Acsq:               uint16(cComputingResource.acsq),
		Cdqm:               uint16(cComputingResource.cdqm),
		CCore:              uint16(cComputingResource.c_core),
		Ffts:               uint16(cComputingResource.ffts),
		Sdma:               uint16(cComputingResource.sdma),
		PcieDma:            uint16(cComputingResource.pcie_dma),
		MemorySize:         uint64(cComputingResource.memory_size),
		EventID:            uint32(cComputingResource.event_id),
		NotifyID:           uint32(cComputingResource.notify_id),
		StreamID:           uint32(cComputingResource.stream_id),
		ModelID:            uint32(cComputingResource.model_id),
		TopicScheduleAicpu: uint16(cComputingResource.topic_schedule_aicpu),
		HostCtrlCPU:        uint16(cComputingResource.host_ctrl_cpu),
		HostAicpu:          uint16(cComputingResource.host_aicpu),
		DeviceAicpu:        uint16(cComputingResource.device_aicpu),
		TopicCtrlCPUSlot:   uint16(cComputingResource.topic_ctrl_cpu_slot),
	}
	return computingResource
}

func convertMediaResource(cMediaResource C.struct_dcmi_media_resource) common.CgoMediaResource {
	mediaResource := common.CgoMediaResource{
		Jpegd: float32(cMediaResource.jpegd),
		Jpege: float32(cMediaResource.jpege),
		Vpc:   float32(cMediaResource.vpc),
		Vdec:  float32(cMediaResource.vdec),
		Pngd:  float32(cMediaResource.pngd),
		Venc:  float32(cMediaResource.venc),
	}
	return mediaResource
}

func convertVDevQueryInfo(cVDevQueryInfo C.struct_dcmi_vdev_query_info) common.CgoVDevQueryInfo {
	name := convertToString(cVDevQueryInfo.name)
	vDevQueryInfo := common.CgoVDevQueryInfo{
		Name:            string(name),
		Status:          uint32(cVDevQueryInfo.status),
		IsContainerUsed: uint32(cVDevQueryInfo.is_container_used),
		Vfid:            uint32(cVDevQueryInfo.vfid),
		VfgID:           uint32(cVDevQueryInfo.vfg_id),
		ContainerID:     uint64(cVDevQueryInfo.container_id),
		Base:            convertBaseResource(cVDevQueryInfo.base),
		Computing:       convertComputingResource(cVDevQueryInfo.computing),
		Media:           convertMediaResource(cVDevQueryInfo.media),
	}
	return vDevQueryInfo
}

func convertVDevQueryStru(cVDevQueryStru C.struct_dcmi_vdev_query_stru) common.CgoVDevQueryStru {
	vDevQueryStru := common.CgoVDevQueryStru{
		VDevID:    uint32(cVDevQueryStru.vdev_id),
		QueryInfo: convertVDevQueryInfo(cVDevQueryStru.query_info),
	}
	return vDevQueryStru
}

// DcGetDeviceVDevResource get virtual device resource info
func (d *DcManager) DcGetDeviceVDevResource(cardID, deviceID int32, vDevID uint32) (common.CgoVDevQueryStru, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.CgoVDevQueryStru{}, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var cMainCmd = C.enum_dcmi_main_cmd(MainCmdVDevMng)
	subCmd := VmngSubCmdGetVDevResource
	var vDevResource C.struct_dcmi_vdev_query_stru
	size := C.uint(unsafe.Sizeof(vDevResource))
	vDevResource.vdev_id = C.uint(vDevID)
	if retCode := C.dcmi_get_device_info(C.int(cardID), C.int(deviceID), cMainCmd, C.uint(subCmd),
		unsafe.Pointer(&vDevResource), &size); int32(retCode) != common.Success {
		return common.CgoVDevQueryStru{}, fmt.Errorf("get device info failed, error is: %d", int32(retCode))
	}
	return convertVDevQueryStru(vDevResource), nil
}

func convertSocTotalResource(cSocTotalResource C.struct_dcmi_soc_total_resource) common.CgoSocTotalResource {
	socTotalResource := common.CgoSocTotalResource{
		VDevNum:   uint32(cSocTotalResource.vdev_num),
		VfgNum:    uint32(cSocTotalResource.vfg_num),
		VfgBitmap: uint32(cSocTotalResource.vfg_bitmap),
		Base:      convertBaseResource(cSocTotalResource.base),
		Computing: convertComputingResource(cSocTotalResource.computing),
		Media:     convertMediaResource(cSocTotalResource.media),
	}
	for i := uint32(0); i < uint32(cSocTotalResource.vdev_num) && i < dcmiMaxVdevNum; i++ {
		socTotalResource.VDevID = append(socTotalResource.VDevID, uint32(cSocTotalResource.vdev_id[i]))
	}
	return socTotalResource
}

// DcGetDeviceTotalResource get device total resource info
func (d *DcManager) DcGetDeviceTotalResource(cardID, deviceID int32) (common.CgoSocTotalResource, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.CgoSocTotalResource{}, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var cMainCmd = C.enum_dcmi_main_cmd(MainCmdVDevMng)
	subCmd := VmngSubCmdGetTotalResource
	var totalResource C.struct_dcmi_soc_total_resource
	size := C.uint(unsafe.Sizeof(totalResource))
	if retCode := C.dcmi_get_device_info(C.int(cardID), C.int(deviceID), cMainCmd, C.uint(subCmd),
		unsafe.Pointer(&totalResource), &size); int32(retCode) != common.Success {
		return common.CgoSocTotalResource{}, fmt.Errorf("get device info failed, error is: %d", int32(retCode))
	}
	if uint32(totalResource.vdev_num) > dcmiMaxVdevNum {
		return common.CgoSocTotalResource{}, fmt.Errorf("get error virtual quantity: %d",
			uint32(totalResource.vdev_num))
	}

	return convertSocTotalResource(totalResource), nil
}

func convertSuperPodInfo(cSuperPodInfo C.struct_dcmi_spod_info) common.CgoSuperPodInfo {
	superPodInfo := common.CgoSuperPodInfo{
		SdId:       uint32(cSuperPodInfo.sdid),
		ScaleType:  uint32(cSuperPodInfo.scale_type),
		SuperPodId: uint32(cSuperPodInfo.super_pod_id),
		ServerId:   uint32(cSuperPodInfo.server_id),
	}

	for i := uint32(0); i < dcmiMaxReserveNum; i++ {
		superPodInfo.Reserve = append(superPodInfo.Reserve, uint32(cSuperPodInfo.reserve[i]))
	}

	return superPodInfo
}

// DcGetSuperPodInfo get device total resource info
func (d *DcManager) DcGetSuperPodInfo(cardID, deviceID int32) (common.CgoSuperPodInfo, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.CgoSuperPodInfo{}, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}

	var unitType C.enum_dcmi_unit_type
	if retCode := C.dcmi_get_device_type(C.int(cardID), C.int(deviceID), &unitType); int32(retCode) != common.Success {
		return common.CgoSuperPodInfo{}, fmt.Errorf("get device type failed, error is: %d", int32(retCode))
	}
	if int32(unitType) != common.NpuType {
		return common.CgoSuperPodInfo{}, fmt.Errorf("not support unit type: %d", int32(unitType))
	}

	var cMainCmd = C.enum_dcmi_main_cmd(MainCmdChipInf)
	subCmd := CinfSubCmdGetSPodInfo
	var sPodInfo C.struct_dcmi_spod_info
	size := C.uint(unsafe.Sizeof(sPodInfo))
	if retCode := C.dcmi_get_device_info(C.int(cardID), C.int(deviceID), cMainCmd, C.uint(subCmd),
		unsafe.Pointer(&sPodInfo), &size); int32(retCode) != common.Success {
		return common.CgoSuperPodInfo{}, fmt.Errorf("get super pod info failed, error is: %d", int32(retCode))
	}

	return convertSuperPodInfo(sPodInfo), nil
}

func convertSocFreeResource(cSocFreeResource C.struct_dcmi_soc_free_resource) common.CgoSocFreeResource {
	socFreeResource := common.CgoSocFreeResource{
		VfgNum:    uint32(cSocFreeResource.vfg_num),
		VfgBitmap: uint32(cSocFreeResource.vfg_bitmap),
		Base:      convertBaseResource(cSocFreeResource.base),
		Computing: convertComputingResource(cSocFreeResource.computing),
		Media:     convertMediaResource(cSocFreeResource.media),
	}
	return socFreeResource
}

// DcGetDeviceFreeResource get device free resource info
func (d *DcManager) DcGetDeviceFreeResource(cardID, deviceID int32) (common.CgoSocFreeResource, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.CgoSocFreeResource{}, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var cMainCmd = C.enum_dcmi_main_cmd(MainCmdVDevMng)
	subCmd := VmngSubCmdGetFreeResource
	var freeResource C.struct_dcmi_soc_free_resource
	size := C.uint(unsafe.Sizeof(freeResource))
	if retCode := C.dcmi_get_device_info(C.int(cardID), C.int(deviceID), cMainCmd, C.uint(subCmd),
		unsafe.Pointer(&freeResource), &size); int32(retCode) != common.Success {
		return common.CgoSocFreeResource{}, fmt.Errorf("get device info failed, error is: %d", int32(retCode))
	}
	return convertSocFreeResource(freeResource), nil
}

// DcGetVDevActivityInfo get vir device activity info by virtual device id
func (d *DcManager) DcGetVDevActivityInfo(cardID, deviceID int32, vDevID uint32) (common.VDevActivityInfo, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.VDevActivityInfo{}, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	if !common.IsValidVDevID(vDevID) {
		return common.VDevActivityInfo{}, fmt.Errorf("vDevID(%d) invalid", vDevID)
	}
	var cMainCmd = C.enum_dcmi_main_cmd(MainCmdVDevMng)
	subCmd := VmngSubCmdGetVDevActivity
	var vDevActivityInfo C.struct_dcmi_vdev_query_stru
	size := C.uint(unsafe.Sizeof(vDevActivityInfo))
	vDevActivityInfo.vdev_id = C.uint(vDevID)
	if retCode := C.dcmi_get_device_info(C.int(cardID), C.int(deviceID), cMainCmd, C.uint(subCmd),
		unsafe.Pointer(&vDevActivityInfo), &size); int32(retCode) != common.Success {
		return common.VDevActivityInfo{}, fmt.Errorf("retCode: %d", int32(retCode))
	}
	totalMemSize := uint64(vDevActivityInfo.query_info.computing.vdev_memory_total)
	usedMemSize := totalMemSize - uint64(vDevActivityInfo.query_info.computing.vdev_memory_free)
	if usedMemSize < 0 {
		return common.VDevActivityInfo{}, errors.New("used memory value abnormal")
	}
	return common.VDevActivityInfo{
		VDevID:         vDevID,
		VDevAiCoreRate: uint32(vDevActivityInfo.query_info.computing.vdev_aicore_utilization),
		VDevTotalMem:   totalMemSize,
		VDevUsedMem:    usedMemSize,
		IsVirtualDev:   true,
	}, nil
}

// DcVGetDeviceInfo get vdevice resource info
func (d *DcManager) DcVGetDeviceInfo(cardID, deviceID int32) (common.VirtualDevInfo, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.VirtualDevInfo{}, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var unitType C.enum_dcmi_unit_type
	if retCode := C.dcmi_get_device_type(C.int(cardID), C.int(deviceID), &unitType); int32(retCode) != common.Success {
		return common.VirtualDevInfo{}, fmt.Errorf("get device type failed, error is: %d", int32(retCode))
	}
	if int32(unitType) != common.NpuType {
		return common.VirtualDevInfo{}, fmt.Errorf("not support unit type: %d", int32(unitType))
	}

	cgoDcmiSocTotalResource, err := d.DcGetDeviceTotalResource(cardID, deviceID)
	if err != nil {
		return common.VirtualDevInfo{}, fmt.Errorf("get device total resource failed, error is: %v", err)
	}

	cgoDcmiSocFreeResource, err := d.DcGetDeviceFreeResource(cardID, deviceID)
	if err != nil {
		return common.VirtualDevInfo{}, fmt.Errorf("get device free resource failed, error is: %v", err)
	}
	dcmiVDevInfo := common.VirtualDevInfo{
		TotalResource: cgoDcmiSocTotalResource,
		FreeResource:  cgoDcmiSocFreeResource,
	}
	for _, vDevID := range cgoDcmiSocTotalResource.VDevID {
		cgoVDevQueryStru, err := d.DcGetDeviceVDevResource(cardID, deviceID, vDevID)
		if err != nil {
			return common.VirtualDevInfo{}, fmt.Errorf("get device virtual resource failed, error is: %v", err)
		}
		dcmiVDevInfo.VDevInfo = append(dcmiVDevInfo.VDevInfo, cgoVDevQueryStru)
		vDevActivityInfo, err := d.DcGetVDevActivityInfo(cardID, deviceID, vDevID)
		if err != nil {
			hwlog.RunLog.Warnf("get cur vDev's activity info failed, err: %s", err)
			continue
		}
		vDevActivityInfo.VDevAiCore = float64(cgoVDevQueryStru.QueryInfo.Computing.Aic)
		dcmiVDevInfo.VDevActivityInfo = append(dcmiVDevInfo.VDevActivityInfo, vDevActivityInfo)
	}
	return dcmiVDevInfo, nil
}

// DcGetCardIDDeviceID get card id and device id from logic id
func (d *DcManager) DcGetCardIDDeviceID(logicID int32) (int32, int32, error) {
	if !common.IsValidLogicIDOrPhyID(logicID) {
		return common.RetError, common.RetError, fmt.Errorf("input invalid logicID: %d", logicID)
	}
	var cardID, deviceID C.int
	if retCode := C.dcmi_get_card_id_device_id_from_logicid(&cardID, &deviceID,
		C.uint(logicID)); int32(retCode) != common.Success {
		return common.RetError, common.RetError,
			fmt.Errorf("failed to get card id and device id by logicID(%d), errorcode is: %d", logicID,
				int32(retCode))
	}
	if !common.IsValidCardIDAndDeviceID(int32(cardID), int32(deviceID)) {
		return common.RetError, common.RetError, fmt.Errorf("failed to get card id and device id, "+
			"cardID(%d) or deviceID(%d) is invalid", int32(cardID), int32(deviceID))
	}

	return int32(cardID), int32(deviceID), nil
}

// DcCreateVDevice create virtual device by logic id
func (d *DcManager) DcCreateVDevice(logicID int32, vDevInfo common.CgoCreateVDevRes) (common.
	CgoCreateVDevOut, error) {
	if !common.IsValidLogicIDOrPhyID(logicID) {
		return common.CgoCreateVDevOut{}, fmt.Errorf("input invalid logicID: %d", logicID)
	}
	cardID, deviceID, err := d.DcGetCardIDDeviceID(logicID)
	if err != nil {
		return common.CgoCreateVDevOut{}, fmt.Errorf("get card id and device id failed, error is: %v", err)
	}

	createVDevOut, err := d.DcCreateVirtualDevice(cardID, deviceID, vDevInfo)
	if err != nil {
		return common.CgoCreateVDevOut{}, fmt.Errorf("create virtual device failed, error is: %v", err)
	}
	return createVDevOut, nil
}

// DcGetVDeviceInfo get virtual device info by logic id
func (d *DcManager) DcGetVDeviceInfo(logicID int32) (common.VirtualDevInfo, error) {
	if !common.IsValidLogicIDOrPhyID(logicID) {
		return common.VirtualDevInfo{}, fmt.Errorf("input invalid logicID: %d", logicID)
	}
	cardID, deviceID, err := d.DcGetCardIDDeviceID(logicID)
	if err != nil {
		return common.VirtualDevInfo{}, fmt.Errorf("get card id and device id failed, error is: %v", err)
	}

	dcmiVDevInfo, err := d.DcVGetDeviceInfo(cardID, deviceID)
	if err != nil {
		return common.VirtualDevInfo{}, fmt.Errorf("get virtual device info failed, error is: %v", err)
	}
	return dcmiVDevInfo, nil
}

// DcDestroyVDevice destroy spec virtual device by logic id
func (d *DcManager) DcDestroyVDevice(logicID int32, vDevID uint32) error {
	if !common.IsValidLogicIDOrPhyID(logicID) {
		return fmt.Errorf("input invalid logicID: %d", logicID)
	}
	cardID, deviceID, err := d.DcGetCardIDDeviceID(logicID)
	if err != nil {
		return fmt.Errorf("get card id and device id failed, error is: %v", err)
	}

	if err = d.DcSetDestroyVirtualDevice(cardID, deviceID, vDevID); err != nil {
		return fmt.Errorf("destroy virtual device failed, error is: %v", err)
	}
	return nil
}

// DcGetDeviceVoltage the accuracy is 0.01v.
func (d *DcManager) DcGetDeviceVoltage(cardID, deviceID int32) (float32, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.RetError, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var vol C.uint
	if retCode := C.dcmi_get_device_voltage(C.int(cardID), C.int(deviceID), &vol); int32(retCode) != common.Success {
		return common.RetError, fmt.Errorf("failed to obtain the voltage based on card_id(%d) and "+
			"device_id(%d), error code: %d", cardID, deviceID, int32(retCode))
	}
	// the voltage's value is error if it's greater than or equal to MaxInt32
	if common.IsGreaterThanOrEqualInt32(int64(vol)) {
		return common.RetError, fmt.Errorf("voltage value out of range(max is int32), "+
			"card_id(%d) and device_id(%d), voltage: %d", cardID, deviceID, int64(vol))
	}

	return float32(vol) * common.ReduceOnePercent, nil
}

// DcGetDevicePowerInfo the accuracy is 0.1w, the result like: 8.2
func (d *DcManager) DcGetDevicePowerInfo(cardID, deviceID int32) (float32, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.RetError, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var cpower C.int
	if retCode := C.dcmi_get_device_power_info(C.int(cardID), C.int(deviceID),
		&cpower); int32(retCode) != common.Success {
		return common.RetError, fmt.Errorf("failed to obtain the power based on card_id(%d) and device_id(%d)"+
			", error code: %d", cardID, deviceID, int32(retCode))
	}
	parsedPower := float32(cpower)
	if parsedPower < 0 {
		return common.RetError, fmt.Errorf("get wrong device power, card_id(%d) and device_id(%d), power: %f",
			cardID, deviceID, parsedPower)
	}

	return parsedPower * common.ReduceTenth, nil

}

// DcGetDeviceFrequency get device frequency, unit MHz
// Ascend910B with frequency type: 2,6,7,9
// Ascend910 with frequency type: 2,6,7,9
// Ascend310 with frequency type: 1,2,6,7,9
// Ascend310P with frequency type: 1,2,7,9,12
// more information see common.DeviceType
func (d *DcManager) DcGetDeviceFrequency(cardID, deviceID int32, devType common.DeviceType) (uint32, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.UnRetError, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var cFrequency C.uint
	if retCode := C.dcmi_get_device_frequency(C.int(cardID), C.int(deviceID), C.enum_dcmi_freq_type(devType.Code),
		&cFrequency); int32(retCode) != common.Success {
		return common.UnRetError,
			buildDcmiErr(cardID, deviceID, fmt.Sprintf("frequency (name: %v, code:%d)", devType.Name, devType.Code), retCode)
	}
	// check whether cFrequency is too big
	if common.IsGreaterThanOrEqualInt32(int64(cFrequency)) || int64(cFrequency) < 0 {
		return common.UnRetError, fmt.Errorf("frequency value out of range [0, int32),card_id(%d) and device_id(%d), "+
			"frequency (name: %v, code:%d): %d", cardID, deviceID, devType.Name, devType.Code, int64(cFrequency))
	}
	return uint32(cFrequency), nil
}

// DcGetMemoryInfo use v3 interface to query memory info
func (d *DcManager) DcGetMemoryInfo(cardID, deviceID int32) (*common.MemoryInfo, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return nil, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var cmInfoV3 CDcmiMemoryInfoV3
	if retCode := C.dcmi_get_device_memory_info_v3(C.int(cardID), C.int(deviceID),
		&cmInfoV3); int32(retCode) != common.Success {
		return nil, fmt.Errorf("failed to obtain the memory info by v3 interface based on card_id("+
			"%d) and device_id(%d), error code: %d", cardID, deviceID, int32(retCode))
	}

	if uint64(cmInfoV3.memory_size) < uint64(cmInfoV3.memory_available) {
		return nil, fmt.Errorf("failed to obtain the memory info by v3 interface based on card_id("+
			"%d) and device_id(%d), total memory is less than available memory", cardID, deviceID)
	}

	return &common.MemoryInfo{
		MemorySize:      uint64(cmInfoV3.memory_size),
		MemoryAvailable: uint64(cmInfoV3.memory_available),
		Frequency:       uint32(cmInfoV3.freq),
		Utilization:     uint32(cmInfoV3.utiliza),
	}, nil

}

// FuncDcmiGetDeviceHbmInfo dcmi_get_device_hbm_info function for outer invoke, only for Ascend910
func FuncDcmiGetDeviceHbmInfo(cardID, deviceID int32) (*common.HbmInfo, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return nil, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var cHbmInfo C.struct_dcmi_hbm_info
	if retCode := C.dcmi_get_device_hbm_info(C.int(cardID), C.int(deviceID),
		&cHbmInfo); int32(retCode) != common.Success {
		return nil, buildDcmiErr(cardID, deviceID, "high bandwidth memory info", retCode)
	}
	hbmTemp := int32(cHbmInfo.temp)
	if hbmTemp < 0 {
		return nil, fmt.Errorf("get wrong device HBM temporary, card_id(%d) and device_id(%d), HBM.temp: %d",
			cardID, deviceID, hbmTemp)
	}
	return &common.HbmInfo{
		MemorySize:        uint64(cHbmInfo.memory_size),
		Frequency:         uint32(cHbmInfo.freq),
		Usage:             uint64(cHbmInfo.memory_usage),
		Temp:              hbmTemp,
		BandWidthUtilRate: uint32(cHbmInfo.bandwith_util_rate)}, nil
}

// DcGetHbmInfo get HBM information A310/A310P not support
func (d *DcManager) DcGetHbmInfo(cardID, deviceID int32) (*common.HbmInfo, error) {
	return &common.HbmInfo{
		MemorySize:        0,
		Frequency:         0,
		Usage:             0,
		Temp:              0,
		BandWidthUtilRate: 0}, nil
}

// DcGetDeviceErrorCode get the error count and errorcode of the device,only return the first errorcode
func (d *DcManager) DcGetDeviceErrorCode(cardID, deviceID int32) (int32, int64, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.RetError, common.RetError, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID,
			deviceID)
	}
	var errCount C.int
	var errCodeArray [common.MaxErrorCodeCount]C.uint
	if retCode := C.dcmi_get_device_errorcode_v2(C.int(cardID), C.int(deviceID), &errCount, &errCodeArray[0],
		common.MaxErrorCodeCount); int32(retCode) != common.Success {
		return common.RetError, common.RetError, fmt.Errorf("failed to obtain the device errorcode based on "+
			"card_id(%d) and device_id(%d), error code: %d, error count: %d", cardID, deviceID, int32(retCode),
			int32(errCount))
	}

	if int32(errCount) < 0 || int32(errCount) > common.MaxErrorCodeCount {
		return common.RetError, common.RetError, fmt.Errorf("get wrong errorcode count, "+
			"card_id(%d) and device_id(%d), errorcode count: %d", cardID, deviceID, int32(errCount))
	}

	return int32(errCount), int64(errCodeArray[0]), nil
}

// DcGetDeviceCount get device count
func (d *DcManager) DcGetDeviceCount() (int32, error) {
	devNum, _, err := d.DcGetLogicIDList()
	if err != nil {
		return common.RetError, fmt.Errorf("get device count failed, error: %v", err)
	}
	return devNum, nil
}

// DcGetLogicIDList get device logic id list
func (d *DcManager) DcGetLogicIDList() (int32, []int32, error) {
	logicIDs := make([]int32, 0)
	var totalNum int32
	_, cardList, err := d.DcGetCardList()
	if err != nil {
		return common.RetError, logicIDs, fmt.Errorf("get card list failed, error: %v", err)
	}
	for _, cardID := range cardList {
		devNumInCard, err := d.DcGetDeviceNumInCard(cardID)
		if err != nil {
			return common.RetError, logicIDs, fmt.Errorf("get device num by cardID: %d failed, error: %v",
				cardID, err)
		}
		totalNum += devNumInCard
		if totalNum > common.HiAIMaxDeviceNum*common.HiAIMaxCardNum {
			return common.RetError, nil, fmt.Errorf("get device num: %d greater than %d",
				totalNum, common.HiAIMaxDeviceNum*common.HiAIMaxCardNum)
		}
		for devID := int32(0); devID < devNumInCard; devID++ {
			logicID, err := d.DcGetDeviceLogicID(cardID, devID)
			if err != nil {
				return common.RetError, nil, fmt.Errorf("get device (cardID: %d, deviceID: %d) logic id "+
					"failed, error: %v", cardID, devID, err)
			}
			logicIDs = append(logicIDs, logicID)
		}
	}
	return totalNum, logicIDs, nil
}

// DcGetDeviceHealth get device health
func (d *DcManager) DcGetDeviceHealth(cardID, deviceID int32) (int32, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.RetError, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var health C.uint
	if retCode := C.dcmi_get_device_health(C.int(cardID), C.int(deviceID),
		&health); int32(retCode) != common.Success {
		return common.RetError, fmt.Errorf("get device (cardID: %d, deviceID: %d) health state failed, ret "+
			"code: %d, health code: %d", cardID, deviceID, int32(retCode), int64(health))
	}
	if common.IsGreaterThanOrEqualInt32(int64(health)) {
		return common.RetError, fmt.Errorf("get wrong health state , device (cardID: %d, deviceID: %d) "+
			"health: %d", cardID, deviceID, int64(health))
	}
	return int32(health), nil
}

// DcGetDeviceUtilizationRate get device utils rate by id
func (d *DcManager) DcGetDeviceUtilizationRate(cardID, deviceID int32, devType common.DeviceType) (int32, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.RetError, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var rate C.uint
	if retCode := C.dcmi_get_device_utilization_rate(C.int(cardID), C.int(deviceID), C.int(devType.Code),
		&rate); int32(retCode) != common.Success {
		return common.RetError,
			buildDcmiErr(cardID, deviceID, fmt.Sprintf("utilization (name: %v, code:%d)", devType.Name, devType.Code), retCode)
	}
	if !common.IsValidUtilizationRate(uint32(rate)) {
		return common.RetError, fmt.Errorf("get wrong device (cardID: %d, deviceID: %d) "+
			"utilization (name: %v, code:%d): %d", cardID, deviceID, devType.Name, devType.Code, uint32(rate))
	}
	return int32(rate), nil
}

// DcGetDeviceTemperature get the device temperature
func (d *DcManager) DcGetDeviceTemperature(cardID, deviceID int32) (int32, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.RetError, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var temp C.int
	if retCode := C.dcmi_get_device_temperature(C.int(cardID), C.int(deviceID),
		&temp); int32(retCode) != common.Success {
		return common.RetError, fmt.Errorf("get device (cardID: %d, deviceID: %d) temperature failed, error "+
			"code is : %d", cardID, deviceID, int32(retCode))
	}
	parsedTemp := int32(temp)
	if parsedTemp < int32(common.DefaultTemperatureWhenQueryFailed) {
		return common.RetError, fmt.Errorf("get wrong device temperature, devcie (cardID: %d, deviceID: %d), "+
			"temperature: %d", cardID, deviceID, parsedTemp)
	}
	return parsedTemp, nil
}

func convertUCharToCharArr(cgoArr [maxChipNameLen]C.uchar) []byte {
	var charArr []byte
	for _, v := range cgoArr {
		if v == 0 {
			break
		}
		charArr = append(charArr, byte(v))
	}
	return charArr
}

// DcGetChipInfo get the chip info by cardID and deviceID
func (d *DcManager) DcGetChipInfo(cardID, deviceID int32) (*common.ChipInfo, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return nil, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var chipInfo C.struct_dcmi_chip_info_v2
	chip := &common.ChipInfo{}
	if rCode := C.dcmi_get_device_chip_info_v2(C.int(cardID), C.int(deviceID), &chipInfo); int32(rCode) != common.Success {
		hwlog.RunLog.Debugf("get device ChipInfo information failed, cardID(%d), deviceID(%d),"+
			" error code: %d", cardID, deviceID, int32(rCode))
		var oldChipInfo C.struct_dcmi_chip_info
		if rCode = C.dcmi_get_device_chip_info(C.int(cardID), C.int(deviceID), &oldChipInfo); int32(rCode) != common.Success {
			return nil, fmt.Errorf("get device ChipInfo information failed, cardID(%d), deviceID(%d),"+
				" error code: %d", cardID, deviceID, int32(rCode))
		}
		chip.Name = string(convertUCharToCharArr(oldChipInfo.chip_name))
		chip.Type = string(convertUCharToCharArr(oldChipInfo.chip_type))
		chip.Version = string(convertUCharToCharArr(oldChipInfo.chip_ver))
		chip.AICoreCnt = int(oldChipInfo.aicore_cnt)
	} else {
		chip.Name = string(convertUCharToCharArr(chipInfo.chip_name))
		chip.Type = string(convertUCharToCharArr(chipInfo.chip_type))
		chip.Version = string(convertUCharToCharArr(chipInfo.chip_ver))
		chip.AICoreCnt = int(chipInfo.aicore_cnt)
		chip.NpuName = string(convertUCharToCharArr(chipInfo.npu_name))
	}
	if !common.IsValidChipInfo(chip) {
		return nil, fmt.Errorf("get device ChipInfo information failed, chip info is empty,"+
			" cardID(%d), deviceID(%d)", cardID, deviceID)
	}

	return chip, nil
}

// DcGetPhysicIDFromLogicID get physicID from logicID
func (d *DcManager) DcGetPhysicIDFromLogicID(logicID int32) (int32, error) {
	if !common.IsValidLogicIDOrPhyID(logicID) {
		return common.RetError, fmt.Errorf("logicID(%d) is invalid", logicID)
	}
	var physicID C.uint
	if rCode := C.dcmi_get_device_phyid_from_logicid(C.uint(logicID), &physicID); int32(rCode) != common.Success {
		return common.RetError, fmt.Errorf("get physic id from logicID(%d) failed, error code: %d", logicID, int32(rCode))
	}
	if !common.IsValidLogicIDOrPhyID(int32(physicID)) {
		return common.RetError, fmt.Errorf("get wrong physicID(%d) from logicID(%d)", uint32(physicID), logicID)
	}
	return int32(physicID), nil
}

// DcGetDeviceIPAddress get device IP address by cardID and deviceID
func (d *DcManager) DcGetDeviceIPAddress(cardID, deviceID, ipType int32) (string, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return "", fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var portType C.enum_dcmi_port_type = 1
	var portID C.int
	var ipAddress C.struct_dcmi_ip_addr
	var maskAddress C.struct_dcmi_ip_addr
	if ipType == ipAddrTypeV6 {
		ipAddress.ip_type = ipAddrTypeV6
	}
	rCode := C.dcmi_get_device_ip(C.int(cardID), C.int(deviceID), portType, portID, &ipAddress, &maskAddress)
	if int32(rCode) != common.Success {
		return "", fmt.Errorf("get device IP address failed, cardID(%d), deviceID(%d), error code: %d",
			cardID, deviceID, int32(rCode))
	}
	if ipType == ipAddrTypeV6 {
		return d.buildIPv6Addr(ipAddress)
	}
	return d.buildIPv4Addr(ipAddress)
}

func (d *DcManager) buildIPv4Addr(ipAddress C.struct_dcmi_ip_addr) (string, error) {
	deviceIP := make([]string, 0, net.IPv4len)
	for key, val := range ipAddress.u_addr {
		if key >= net.IPv4len {
			break
		}
		deviceIP = append(deviceIP, fmt.Sprintf("%v", val))
	}
	if netIP := net.ParseIP(strings.Join(deviceIP, ".")); netIP != nil {
		return netIP.String(), nil
	}
	return "", fmt.Errorf("the device IPv4 address is invalid, value: %v", deviceIP)
}

func (d *DcManager) buildIPv6Addr(ipAddress C.struct_dcmi_ip_addr) (string, error) {
	deviceIP := make([]byte, 0, net.IPv6len)
	for key, val := range ipAddress.u_addr {
		if key >= net.IPv6len {
			break
		}
		deviceIP = append(deviceIP, byte(val))
	}
	if netIP := net.IP(deviceIP); netIP != nil {
		return netIP.String(), nil
	}
	return "", fmt.Errorf("the device IPv6 address is invalid, value: %v", deviceIP)
}

func callDcmiGetDeviceNetworkHealth(cardID, deviceID int32, result chan<- common.DeviceNetworkHealth) {
	var healthCode C.enum_dcmi_rdfx_detect_result
	rCode := C.dcmi_get_device_network_health(C.int(cardID), C.int(deviceID), &healthCode)
	result <- common.DeviceNetworkHealth{HealthCode: uint32(healthCode), RetCode: int32(rCode)}
}

// DcGetDeviceNetWorkHealth get device network health by cardID and deviceID
func (d *DcManager) DcGetDeviceNetWorkHealth(cardID, deviceID int32) (uint32, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.UnRetError, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}

	result := make(chan common.DeviceNetworkHealth, 1)
	go callDcmiGetDeviceNetworkHealth(cardID, deviceID, result)
	select {
	case res := <-result:
		if res.RetCode != common.Success {
			return common.UnRetError, fmt.Errorf("get device network healthCode failed, cardID(%d),"+
				" deviceID(%d), ret code: %d, health code: %d", cardID, deviceID, res.RetCode, res.HealthCode)
		}

		if int32(res.HealthCode) < 0 || int32(res.HealthCode) > int32(math.MaxInt8) {
			return common.UnRetError, fmt.Errorf("get wrong device network healthCode, cardID(%d), deviceID(%d),"+
				" error healthCode: %d", cardID, deviceID, int32(res.HealthCode))
		}

		return res.HealthCode, nil
	// dcmi_get_device_network_health is occasionally blocked for a long time, because of retrying,
	// after the card dropped. This method is used to interrupt the execution of the dcmi interface,
	// if invoking time excceeds 1 second.
	case <-time.After(common.DcmiApiTimeout * time.Second):
		return common.UnRetError, fmt.Errorf("accessing dcmi_get_device_network_health interface timeout, "+
			"cardID(%d), deviceID(%d)", cardID, deviceID)
	}
}

// DcGetLogicIDFromPhysicID get logicID from physicID
func (d *DcManager) DcGetLogicIDFromPhysicID(physicID int32) (int32, error) {
	if !common.IsValidLogicIDOrPhyID(physicID) {
		return common.RetError, fmt.Errorf("physicID(%d) is invalid", physicID)
	}
	var logicID C.uint
	if rCode := C.dcmi_get_device_logicid_from_phyid(C.uint(physicID), &logicID); int32(rCode) != common.Success {
		return common.RetError, fmt.Errorf("get logicID from physicID(%d) failed, error code: %d",
			physicID, int32(rCode))
	}

	if !common.IsValidLogicIDOrPhyID(int32(logicID)) {
		return common.RetError, fmt.Errorf("get wrong logicID(%d) from physicID(%d)", uint32(logicID), physicID)
	}
	return int32(logicID), nil
}

// FuncDcmiMcuGetPowerInfo dcmi_mcu_get_power_info_new function for outer invoke
func FuncDcmiMcuGetPowerInfo(cardID int32) (float32, error) {
	var power C.int
	if retCode := C.dcmi_mcu_get_power_info_new(C.int(cardID), &power); int32(retCode) != common.Success {
		return common.RetError, fmt.Errorf("mcu_get_power_info failed, error code is:%d", int32(retCode))
	}
	parsedPower := float32(power)
	if parsedPower < 0 {
		return common.RetError, fmt.Errorf("get wrong mcu_get_power_info, cardID: %d, power: %f", cardID,
			parsedPower)
	}
	return parsedPower * common.ReduceTenth, nil
}

// DcGetMcuPowerInfo this function is only for Ascend310P, A910/A310 not support
func (d *DcManager) DcGetMcuPowerInfo(cardID int32) (float32, error) {
	return 0, nil
}

// DcGetProductType get product type by dcmi interface
func (d *DcManager) DcGetProductType(cardID, deviceID int32) (string, error) {
	cProductType := C.CString(string(make([]byte, productTypeLen)))
	defer C.free(unsafe.Pointer(cProductType))
	err := C.dcmi_get_product_type(C.int(cardID), C.int(deviceID), (*C.char)(cProductType), productTypeLen+1)
	if err != 0 {
		return "", fmt.Errorf("get product type failed, errCode: %d", int32(err))
	}
	return C.GoString(cProductType), nil
}

// DcGetNpuWorkMode get npu work mode, this function is only for Ascend910, A310/310P not support
func (d *DcManager) DcGetNpuWorkMode(cardID int32) (int, error) {
	var cWorkMode C.uchar
	err := C.dcmi_get_npu_work_mode(C.int(cardID), &cWorkMode)
	if err != 0 {
		return common.RetError, fmt.Errorf("get npu work mode failed, errCode: %d", int32(err))
	}
	return int(cWorkMode), nil
}

// DcSetDeviceReset reset spec device chip
func (d *DcManager) DcSetDeviceReset(cardID, deviceID int32) error {
	var channelType C.enum_dcmi_reset_channel = C.INBAND_CHANNEL
	return d.setDeviceReset(cardID, deviceID, channelType)
}

// DcGetBrotherCardID get brother card id
func (d *DcManager) DcGetBrotherCardID(cardID, deviceID int32) (int32, error) {
	var broCardID C.int
	errCode := C.dcmi_get_netdev_brother_device(C.int(cardID), C.int(deviceID), &broCardID)
	if errCode != common.Success {
		return common.RetError, fmt.Errorf("unable to get brother card, errCode: %v", errCode)
	}
	return int32(broCardID), nil
}

// DcGetOutBandChannelState get out band channel state
func (d *DcManager) DcGetOutBandChannelState(cardID, deviceID int32) error {
	var channelState C.int
	errCode := C.dcmi_get_device_outband_channel_state(C.int(cardID), C.int(deviceID), &channelState)
	if errCode != common.Success {
		return fmt.Errorf("get out band channel state error, errCode: %v", errCode)
	}
	if channelState != common.ChannelStateOk {
		return fmt.Errorf("chip reset not support, channel state: %v", channelState)
	}
	return nil
}

// DcPreResetSoc pre reset soc, used before reset out band
func (d *DcManager) DcPreResetSoc(cardID, deviceID int32) error {
	errCode := C.dcmi_pre_reset_soc(C.int(cardID), C.int(deviceID))
	if errCode != common.Success {
		return fmt.Errorf("pre reset failed, cardID: %v, deviceID: %v, errCode: %v", cardID, deviceID, errCode)
	}
	return nil
}

// DcSetDeviceResetOutBand reset spec device chip out band
func (d *DcManager) DcSetDeviceResetOutBand(cardID, deviceID int32) error {
	var channelType C.enum_dcmi_reset_channel = C.OUTBAND_CHANNEL
	return d.setDeviceReset(cardID, deviceID, channelType)
}

// DcRescanSoc trigger soc rescan, non-blocking
func (d *DcManager) DcRescanSoc(cardID, deviceID int32) error {
	errCode := C.dcmi_rescan_soc(C.int(cardID), C.int(deviceID))
	if errCode != common.Success {
		return fmt.Errorf("fail to rescan chip cardID %d, deviceID %v, errCode: %v", cardID, deviceID, errCode)
	}
	return nil
}

func (d *DcManager) setDeviceReset(cardID, deviceID int32, channelType C.enum_dcmi_reset_channel) error {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	if errCode := C.dcmi_set_device_reset(C.int(cardID), C.int(deviceID), channelType); errCode != 0 {
		return fmt.Errorf("cardID(%d) and deviceID(%d) hot reset errCode: %v", cardID, deviceID, errCode)
	}
	return nil
}

// DcGetDeviceBootStatus get NPU boot status
func (d *DcManager) DcGetDeviceBootStatus(logicID int32) (int, error) {
	if !common.IsValidLogicIDOrPhyID(logicID) {
		return common.RetError, fmt.Errorf("input invalid logicID: %d", logicID)
	}
	cardID, deviceID, err := d.DcGetCardIDDeviceID(logicID)
	if err != nil {
		return common.RetError, fmt.Errorf("failed to get cardID and deviceID by logicID(%d)", logicID)
	}
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.RetError, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var bootStatus C.enum_dcmi_boot_status = C.DCMI_BOOT_STATUS_FINISH
	if errCode := C.dcmi_get_device_boot_status(C.int(cardID), C.int(deviceID), &bootStatus); errCode != 0 {
		return common.RetError, fmt.Errorf("device boot status errCode: %v", errCode)
	}
	return int(bootStatus), nil
}

// DcGetDeviceAllErrorCode get the error count and all error codes of the device
func (d *DcManager) DcGetDeviceAllErrorCode(cardID, deviceID int32) (int32, []int64, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.RetError, nil, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID,
			deviceID)
	}
	var errCount C.int
	var errCodeArray [common.MaxErrorCodeCount]C.uint
	retCode := C.dcmi_get_device_errorcode_v2(C.int(cardID), C.int(deviceID), &errCount, &errCodeArray[0],
		common.MaxErrorCodeCount)

	var health C.uint
	healthRetCode := C.dcmi_get_device_health(C.int(cardID), C.int(deviceID), &health)

	if int32(retCode) != common.Success && int32(healthRetCode) != common.DeviceNotReadyErrCode {
		return common.RetError, nil, fmt.Errorf("failed to obtain the device errorcode based on cardID("+
			"%d) and deviceID(%d), error code: %d, error count: %d", cardID, deviceID, int32(retCode), int32(errCount))
	}

	errCodes := make([]int64, 0, len(errCodeArray))
	for _, errCode := range errCodeArray {
		if int64(errCode) != 0 {
			errCodes = append(errCodes, int64(errCode))
		}
	}

	if int32(healthRetCode) == common.DeviceNotReadyErrCode {
		hwlog.RunLog.Errorf("device errorcode v2 ret code: %d, device health ret code: %d, device not ready, "+
			"maybe a card drop fault occurred on cardID(%d) and deviceID(%d)", int32(retCode), int32(healthRetCode),
			cardID, deviceID)
		errCount += 1
		errCodes = append(errCodes, common.CardDropFaultCode)
	}

	if int32(errCount) < 0 || int32(errCount) > common.MaxErrorCodeCount {
		return common.RetError, nil, fmt.Errorf("get wrong errorcode count, "+
			"cardID(%d) and deviceID(%d), errorcode count: %d", cardID, deviceID, int32(errCount))
	}

	return int32(errCount), errCodes, nil
}

// DcSubscribeDeviceFaultEvent subscribe device fault, callback with func 'faultEventCallFunc'
func (d *DcManager) DcSubscribeDeviceFaultEvent(cardID, deviceID int32) error {
	if faultEventCallFunc == nil {
		return errors.New("callFunc is invalid, can't start subscribe")
	}

	var filter C.struct_dcmi_event_filter
	if rCode := C.dcmi_subscribe_fault_event(C.int(cardID), C.int(deviceID), filter); int32(rCode) != common.Success {
		return fmt.Errorf("subscribe fault event failed, cardID(%d) and deviceID(%d), error code: %d",
			cardID, deviceID, int32(rCode))
	}
	return nil
}

// DcSetFaultEventCallFunc set fault event call back func
func (d *DcManager) DcSetFaultEventCallFunc(businessFunc func(common.DevFaultInfo)) {
	faultEventCallFunc = businessFunc
}

//export goEventFaultCallBack
func goEventFaultCallBack(event C.struct_dcmi_dms_fault_event) {
	if faultEventCallFunc == nil {
		hwlog.RunLog.Errorf("no fault event call back func")
		return
	}
	// recovery event recorded fault event occurrence time, the recovery event time cannot be obtained.
	// Therefore, all event occurrence time is recorded as the current host time when the event is received.
	devFaultInfo := common.DevFaultInfo{
		EventID:         int64(event.event_id),
		LogicID:         int32(event.deviceid),
		ModuleType:      int8(event.node_type),
		ModuleID:        int8(event.node_id),
		SubModuleType:   int8(event.sub_node_type),
		SubModuleID:     int8(event.sub_node_id),
		Severity:        int8(event.severity),
		Assertion:       int8(event.assertion),
		AlarmRaisedTime: time.Now().UnixMilli(),
	}
	faultEventCallFunc(devFaultInfo)
}

// DcGetDieID get chip die ID, like VDieID or NDieID, only Ascend910 has NDieID
func (d *DcManager) DcGetDieID(cardID, deviceID int32, dcmiDieType DieType) (string, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return "", fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}

	if dcmiDieType != VDIE && dcmiDieType != NDIE {
		return "", fmt.Errorf("dcmi die type can only be one of %d or %d", VDIE, NDIE)
	}

	var dieIDObj C.struct_dcmi_die_id
	if retCode := C.dcmi_get_device_die_v2(C.int(cardID), C.int(deviceID),
		C.enum_dcmi_die_type(dcmiDieType), &dieIDObj); int32(retCode) != common.Success {
		return "", buildDcmiErr(cardID, deviceID, "chip die ID", retCode)
	}

	const hexBase = 16
	dieIDStr := make([]string, DieIDCount)

	hwlog.RunLog.Debugf("cardID(%d), deviceID(%d) get die type(%d) value %v", cardID, deviceID, dcmiDieType,
		dieIDObj.soc_die)
	for i := 0; i < DieIDCount; i++ {
		s := strconv.FormatUint(uint64(dieIDObj.soc_die[i]), hexBase)
		// Each part of the die id consists of 8 characters, and if the length is not enough,
		// zero is added at the beginning
		dieIDStr[i] = fmt.Sprintf("%08s", s)
	}
	return strings.ToUpper(strings.Join(dieIDStr, "-")), nil
}

// DcGetDevProcessInfo chip process info
func (d *DcManager) DcGetDevProcessInfo(cardID, deviceID int32) (*common.DevProcessInfo, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return nil, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}

	var procList [common.MaxProcNum]C.struct_dcmi_proc_mem_info
	var procNum C.int

	if retCode := C.dcmi_get_device_resource_info(C.int(cardID), C.int(deviceID), &procList[0],
		&procNum); int32(retCode) != common.Success {
		return nil, buildDcmiErr(cardID, deviceID, "device resource", retCode)
	}

	if int32(procNum) < 0 || int32(procNum) > common.MaxProcNum {
		return nil, fmt.Errorf("get invalid proccess num (%d), cardID(%d) and deviceID(%d)", int32(procNum), cardID,
			deviceID)
	}

	return convertToDevResourceInfo(procList, int32(procNum)), nil
}

func convertToDevResourceInfo(procList [common.MaxProcNum]C.struct_dcmi_proc_mem_info,
	procNum int32) *common.DevProcessInfo {
	if procNum < 0 || procNum > common.MaxProcNum {
		hwlog.RunLog.Errorf("process num %v is not within in the range [0~%v]", procNum, common.MaxProcNum)
		return nil
	}

	info := new(common.DevProcessInfo)
	if procNum == 0 {
		return info
	}

	info.ProcNum = procNum
	for i := int32(0); i < procNum; i++ {
		proc := common.DevProcInfo{
			Pid:      int32(procList[i].proc_id),
			MemUsage: float64(procList[i].proc_mem_usage) / common.UnitMB, // convert byte to MB
		}
		info.DevProcArray = append(info.DevProcArray, proc)
	}

	return info
}

// DcGetPCIeBusInfo pcie bus info
func (d *DcManager) DcGetPCIeBusInfo(cardID, deviceID int32) (string, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return "", fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}

	var pcieInfo C.struct_dcmi_pcie_info_all

	if retCode := C.dcmi_get_device_pcie_info_v2(C.int(cardID),
		C.int(deviceID), &pcieInfo); int32(retCode) != common.Success {
		return "", buildDcmiErr(cardID, deviceID, "pcie bus", retCode)
	}

	info := fmt.Sprintf("%04X:%02X:%02X.%-4X", int32(pcieInfo.domain), uint32(pcieInfo.bdf_busid),
		uint32(pcieInfo.bdf_deviceid), uint32(pcieInfo.bdf_funcid))
	hwlog.RunLog.Debugf("pcie bus info is: '%s'", info)

	return strings.TrimRight(info, " "), nil
}

// DcGetDeviceBoardInfo return board info of device
func (d *DcManager) DcGetDeviceBoardInfo(cardID, deviceID int32) (common.BoardInfo, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.BoardInfo{}, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}

	var cBoardInfo C.struct_dcmi_board_info

	if retCode := C.dcmi_get_device_board_info(C.int(cardID), C.int(deviceID),
		&cBoardInfo); int32(retCode) != common.Success {
		return common.BoardInfo{}, buildDcmiErr(cardID, deviceID, "board info", retCode)
	}

	return common.BoardInfo{
		BoardId: uint32(cBoardInfo.board_id),
		PcbId:   uint32(cBoardInfo.pcb_id),
		BomId:   uint32(cBoardInfo.bom_id),
		SlotId:  uint32(cBoardInfo.slot_id),
	}, nil
}

// DcGetPCIEBandwidth get pcie bandwidth value
func (d *DcManager) DcGetPCIEBandwidth(cardID, deviceID int32, profilingTime int) (common.PCIEBwStat, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.PCIEBwStat{}, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var dcmiPCIEBandwidth C.struct_dcmi_pcie_link_bandwidth_info
	var pcieBandwidth common.PCIEBwStat
	dcmiPCIEBandwidth.profiling_time = C.int(profilingTime)
	retCode := C.dcmi_get_pcie_link_bandwidth_info(C.int(cardID), C.int(deviceID), &dcmiPCIEBandwidth)
	if int32(retCode) != common.Success {
		return pcieBandwidth, buildDcmiErr(cardID, deviceID, "PCIEBandwidth", retCode)
	}

	pcieBandwidth.PcieRxPBw = d.convertPcieBw(dcmiPCIEBandwidth.rx_p_bw)
	pcieBandwidth.PcieRxNPBw = d.convertPcieBw(dcmiPCIEBandwidth.rx_np_bw)
	pcieBandwidth.PcieRxCPLBw = d.convertPcieBw(dcmiPCIEBandwidth.rx_cpl_bw)

	pcieBandwidth.PcieTxPBw = d.convertPcieBw(dcmiPCIEBandwidth.tx_p_bw)
	pcieBandwidth.PcieTxNPBw = d.convertPcieBw(dcmiPCIEBandwidth.tx_np_bw)
	pcieBandwidth.PcieTxCPLBw = d.convertPcieBw(dcmiPCIEBandwidth.tx_cpl_bw)

	return pcieBandwidth, nil
}

func (d *DcManager) convertPcieBw(pcieBwArr [agentdrvProfDataNum]C.uint) common.PcieStatValue {
	return common.PcieStatValue{
		PcieMinBw: int32(pcieBwArr[0]),
		PcieMaxBw: int32(pcieBwArr[1]),
		PcieAvgBw: int32(pcieBwArr[agentdrvProfDataNum-1]),
	}
}

// DcGetDcmiVersion return dcmi version
func (d *DcManager) DcGetDcmiVersion() (string, error) {
	cDcmiVer := C.CString(string(make([]byte, dcmiVersionLen)))
	defer C.free(unsafe.Pointer(cDcmiVer))
	if retCode := C.dcmi_get_dcmi_version((*C.char)(cDcmiVer), dcmiVersionLen+1); int32(retCode) != common.Success {
		return "", fmt.Errorf("get dcmi version failed, errCode: %d", int32(retCode))
	}
	return C.GoString(cDcmiVer), nil
}

// DcGetDeviceEccInfo get ECC info
func (d *DcManager) DcGetDeviceEccInfo(cardID, deviceID int32, inputType common.DcmiDeviceType) (
	*common.ECCInfo, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return nil, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	dcmiDeviceType, err := d.getInputType(inputType)
	if err != nil {
		return nil, err
	}
	var deviceEccInfo C.struct_dcmi_ecc_info
	if retCode := C.dcmi_get_device_ecc_info(C.int(cardID), C.int(deviceID), dcmiDeviceType,
		&deviceEccInfo); retCode != 0 {
		return nil, buildDcmiErr(cardID, deviceID, "dcmi device ECC", retCode)
	}
	eccInfo := &common.ECCInfo{
		EnableFlag:                int32(deviceEccInfo.enable_flag),
		SingleBitErrorCnt:         int64(deviceEccInfo.single_bit_error_cnt),
		DoubleBitErrorCnt:         int64(deviceEccInfo.double_bit_error_cnt),
		TotalSingleBitErrorCnt:    int64(deviceEccInfo.total_single_bit_error_cnt),
		TotalDoubleBitErrorCnt:    int64(deviceEccInfo.total_double_bit_error_cnt),
		SingleBitIsolatedPagesCnt: int64(deviceEccInfo.single_bit_isolated_pages_cnt),
		DoubleBitIsolatedPagesCnt: int64(deviceEccInfo.double_bit_isolated_pages_cnt),
	}
	return eccInfo, nil
}

// DcGetHccsStatisticInfo get HCCS statistic info
func (d *DcManager) DcGetHccsStatisticInfo(cardID, deviceID int32) (common.HccsStatisticInfo, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.HccsStatisticInfo{}, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var cMainCmd = C.enum_dcmi_main_cmd(MainCmdHccs)
	subCmd := HccsSubCmdGetStatisticInfo
	var hccsStatisticInfo C.struct_dcmi_hccs_statistic_info
	// Use a secure function to get the address (for cleanCode)
	addr, err := getAddrWithOffset(unsafe.Pointer(&hccsStatisticInfo), unsafe.Sizeof(hccsStatisticInfo), 0)
	if err != nil {
		return common.HccsStatisticInfo{}, fmt.Errorf("get hccsStatisticInfo addr failed, error is: %v", err)
	}
	size := C.uint(unsafe.Sizeof(hccsStatisticInfo))
	if retCode := C.dcmi_get_device_info(C.int(cardID), C.int(deviceID), cMainCmd, C.uint(subCmd),
		addr, &size); int32(retCode) != common.Success {
		return common.HccsStatisticInfo{}, buildDcmiErr(cardID, deviceID, "hccs statistic", retCode)
	}
	return convertHccsStatisticInfoStruct(hccsStatisticInfo), nil
}

// DcGetHccsStatisticInfoU64 get HCCS statistic info
func (d *DcManager) DcGetHccsStatisticInfoU64(cardID, deviceID int32) (common.HccsStatisticInfo, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.HccsStatisticInfo{}, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var cMainCmd = C.enum_dcmi_main_cmd(MainCmdHccs)
	subCmd := HccsSubCmdGetStatisticInfoU64
	var hccsStatisticInfo C.struct_dcmi_hccs_statistic_info_u64
	// Use a secure function to get the address (for cleanCode)
	addr, err := getAddrWithOffset(unsafe.Pointer(&hccsStatisticInfo), unsafe.Sizeof(hccsStatisticInfo), 0)
	if err != nil {
		return common.HccsStatisticInfo{}, fmt.Errorf("get hccsStatisticInfo addr failed, error is: %v", err)
	}
	size := C.uint(unsafe.Sizeof(hccsStatisticInfo))
	if retCode := C.dcmi_get_device_info(C.int(cardID), C.int(deviceID), cMainCmd, C.uint(subCmd),
		addr, &size); int32(retCode) != common.Success {
		return common.HccsStatisticInfo{}, buildDcmiErr(cardID, deviceID, "hccs statistic", retCode)
	}
	return convertHccsStatisticInfoStructU64(hccsStatisticInfo), nil
}

func convertHccsStatisticInfoStruct(hccsStatisticInfo C.struct_dcmi_hccs_statistic_info) common.HccsStatisticInfo {
	cgoHccsStatisticInfo := common.HccsStatisticInfo{}
	for i := uint32(0); i < dcmiHccsMaxPcsNum; i++ {
		cgoHccsStatisticInfo.TxCnt = append(cgoHccsStatisticInfo.TxCnt, uint64(hccsStatisticInfo.tx_cnt[i]))
		cgoHccsStatisticInfo.CrcErrCnt = append(cgoHccsStatisticInfo.CrcErrCnt, uint64(hccsStatisticInfo.crc_err_cnt[i]))
		cgoHccsStatisticInfo.RxCnt = append(cgoHccsStatisticInfo.RxCnt, uint64(hccsStatisticInfo.rx_cnt[i]))
	}
	return cgoHccsStatisticInfo
}

func convertHccsStatisticInfoStructU64(hccsStatisticInfo C.struct_dcmi_hccs_statistic_info_u64) common.HccsStatisticInfo {
	cgoHccsStatisticInfo := common.HccsStatisticInfo{}
	for i := uint32(0); i < dcmiHccsMaxPcsNum; i++ {
		cgoHccsStatisticInfo.TxCnt = append(cgoHccsStatisticInfo.TxCnt, uint64(hccsStatisticInfo.tx_cnt[i]))
		cgoHccsStatisticInfo.CrcErrCnt = append(cgoHccsStatisticInfo.CrcErrCnt, uint64(hccsStatisticInfo.crc_err_cnt[i]))
		cgoHccsStatisticInfo.RxCnt = append(cgoHccsStatisticInfo.RxCnt, uint64(hccsStatisticInfo.rx_cnt[i]))
	}
	return cgoHccsStatisticInfo
}

// DcGetHccsBandwidthInfo get HCCS bandwidth info
func (d *DcManager) DcGetHccsBandwidthInfo(cardID int32, deviceID int32,
	profilingTime int) (common.HccsBandwidthInfo, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.HccsBandwidthInfo{}, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var hccsBandwidthInfo C.struct_dcmi_hccs_bandwidth_info
	hccsBandwidthInfo.profiling_time = C.int(profilingTime)
	if retCode := C.dcmi_get_hccs_link_bandwidth_info(C.int(cardID), C.int(deviceID),
		&hccsBandwidthInfo); int32(retCode) != common.Success {
		return common.HccsBandwidthInfo{}, buildDcmiErr(cardID, deviceID, "hccs bandwidth", retCode)
	}
	return convertHccsBandwidthInfoStruct(hccsBandwidthInfo), nil
}

func convertHccsBandwidthInfoStruct(hccsBandwidthInfo C.struct_dcmi_hccs_bandwidth_info) common.HccsBandwidthInfo {
	cgoHccsBWInfo := common.HccsBandwidthInfo{}
	cgoHccsBWInfo.ProfilingTime = uint32(hccsBandwidthInfo.profiling_time)
	cgoHccsBWInfo.TotalTxbw = float64(hccsBandwidthInfo.total_txbw)
	cgoHccsBWInfo.TotalRxbw = float64(hccsBandwidthInfo.total_rxbw)
	for i := uint32(0); i < dcmiHccsMaxPcsNum; i++ {
		cgoHccsBWInfo.TxBandwidth = append(cgoHccsBWInfo.TxBandwidth, float64(hccsBandwidthInfo.tx_bandwidth[i]))
		cgoHccsBWInfo.RxBandwidth = append(cgoHccsBWInfo.RxBandwidth, float64(hccsBandwidthInfo.rx_bandwidth[i]))
	}
	return cgoHccsBWInfo
}

// DcGetSioInfo get SIO info
func (d *DcManager) DcGetSioInfo(cardID, deviceID int32) (common.SioCrcErrStatisticInfo, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return common.SioCrcErrStatisticInfo{}, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var cMainCmd = C.enum_dcmi_main_cmd(MainCmdSio)
	subCmd := SioSubCmdCrcErrStatistics
	var sioInfo C.struct_dcmi_sio_crc_err_statistic_info
	// Use a secure function to get the address (for cleanCode)
	addr, err := getAddrWithOffset(unsafe.Pointer(&sioInfo), unsafe.Sizeof(sioInfo), 0)
	if err != nil {
		return common.SioCrcErrStatisticInfo{}, fmt.Errorf("get sioInfo addr failed, error is: %v", err)
	}
	size := C.uint(unsafe.Sizeof(sioInfo))
	if retCode := C.dcmi_get_device_info(C.int(cardID), C.int(deviceID), cMainCmd, C.uint(subCmd),
		addr, &size); int32(retCode) != common.Success {
		return common.SioCrcErrStatisticInfo{}, buildDcmiErr(cardID, deviceID, "super pod sio", retCode)
	}
	return convertSioInfoStruct(sioInfo), nil
}

func convertSioInfoStruct(sPodSioInfo C.struct_dcmi_sio_crc_err_statistic_info) common.SioCrcErrStatisticInfo {
	cgoSPodSioInfo := common.SioCrcErrStatisticInfo{
		TxErrCnt: int64(sPodSioInfo.tx_error_count),
		RxErrCnt: int64(sPodSioInfo.rx_error_count),
	}
	for i := uint32(0); i < dcmiMaxReserveNum; i++ {
		cgoSPodSioInfo.Reserved = append(cgoSPodSioInfo.Reserved, uint32(sPodSioInfo.reserved[i]))
	}
	return cgoSPodSioInfo
}

func (d *DcManager) getInputType(inputType common.DcmiDeviceType) (C.enum_dcmi_device_type, error) {
	switch inputType {
	case common.DcmiDeviceTypeDDR:
		return C.DCMI_DEVICE_TYPE_DDR, nil
	case common.DcmiDeviceTypeSRAM:
		return C.DCMI_DEVICE_TYPE_SRAM, nil
	case common.DcmiDeviceTypeHBM:
		return C.DCMI_DEVICE_TYPE_HBM, nil
	case common.DcmiDeviceTypeNPU:
		return C.DCMI_DEVICE_TYPE_NPU, nil
	case common.DcmiDeviceTypeNONE:
		return C.DCMI_DEVICE_TYPE_NONE, nil
	default:
		return C.DCMI_DEVICE_TYPE_NONE, fmt.Errorf("invalid input type for getting device ecc info")
	}
}

// Define a safe function to get address offsets (for cleanCode)
func getAddrWithOffset(addr unsafe.Pointer, length uintptr, offset uintptr) (unsafe.Pointer, error) {
	if offset > length {
		return nil, fmt.Errorf("offset(%d) is invalid, length(%d)", offset, length)
	}
	return (unsafe.Pointer)(uintptr(addr) + offset), nil
}

// DcGetDeviceMainBoardInfo return mainboardId of device
func (d *DcManager) DcGetDeviceMainBoardInfo(cardID, deviceID int32) (uint32, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return 0, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var cMainBoardId C.uint
	if retCode := C.dcmi_get_mainboard_id(C.int(cardID), C.int(deviceID),
		&cMainBoardId); int32(retCode) != common.Success {
		return 0, buildDcmiErr(cardID, deviceID, "mainBoardId", retCode)
	}

	return uint32(cMainBoardId), nil
}
func buildDcmiErr(cardID, deviceID int32, msg string, errCode C.int) error {
	errDesc, ok := dcmiErrMap[int32(errCode)]
	if !ok {
		errDesc = "unknown error code"
	}
	return fmt.Errorf("cardID(%d),deviceID(%d):get %s info failed,error code: %v,error desc: %v",
		cardID, deviceID, msg, errCode, errDesc)
}

// DcGetSuperPodStatus get super pod status
func (d *DcManager) DcGetSuperPodStatus(cardID, deviceID int32, sdid uint32) (int, error) {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return 0, fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	var status C.uint
	if retCode := C.dcmi_get_spod_node_status(C.int(cardID), C.int(deviceID),
		C.unsigned(sdid), &status); int32(retCode) != common.Success {
		return 0, buildDcmiErr(cardID, deviceID, "GetSuperPodStatus", retCode)
	}
	return int(status), nil
}

// DcSetSuperPodStatus set super pod status
func (d *DcManager) DcSetSuperPodStatus(cardID, deviceID int32, sdid, status uint32) error {
	if !common.IsValidCardIDAndDeviceID(cardID, deviceID) {
		return fmt.Errorf("cardID(%d) or deviceID(%d) is invalid", cardID, deviceID)
	}
	if retCode := C.dcmi_set_spod_node_status(C.int(cardID), C.int(deviceID),
		C.uint(sdid), C.uint(status)); int32(retCode) != common.Success {
		return buildDcmiErr(cardID, deviceID, "DcSetSuperPodStatus", retCode)
	}
	return nil
}

// DcGetCardElabelV2 get card elabel information
func (d *DcManager) DcGetCardElabelV2(cardID int32) (common.ElabelInfo, error) {
	if !common.IsValidCardID(cardID) {
		return common.ElabelInfo{}, fmt.Errorf("cardID(%d) is invalid", cardID)
	}
	var elabelInfo C.struct_dcmi_elabel_info
	if retCode := C.dcmi_get_card_elabel_v2(C.int(cardID), &elabelInfo); int32(retCode) != common.Success {
		return common.ElabelInfo{}, fmt.Errorf("cardID(%d): get elabel info failed, error code: %v", cardID, retCode)
	}
	return common.ElabelInfo{
		ProductName:      C.GoString(&elabelInfo.product_name[0]),
		Model:            C.GoString(&elabelInfo.model[0]),
		Manufacturer:     C.GoString(&elabelInfo.manufacturer[0]),
		ManufacturerDate: C.GoString(&elabelInfo.manufacturer_date[0]),
		SerialNumber:     C.GoString(&elabelInfo.serial_number[0]),
	}, nil
}
