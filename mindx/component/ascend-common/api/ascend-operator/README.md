# ascend-aperator-apis

## 介绍

ascend-aperator-apis旨在为用户提供AscendJob API，及其Clientsets, Listers、Informers。使用户能轻松对AscendJob进行CRUD操作。

## 接口说明

1. 创建clientsets

   ```go
   NewForConfig(c *rest.Config)(*Clientset, error) 
   ```

   | Parameters | Input/Output | Parameter Type | Description                                                  |
   | ---------- | ------------ | -------------- | ------------------------------------------------------------ |
   | c          | Input        | *rest.Config   | 客户端配置文件，由k8s提供的接口生成。包括cluster host、证书等信息 |
   | -          | Output       | *clientsets    | Client集合，包括AscendJob client和discovery client           |
   | -          | Output       | error          | 错误信息                                                     |

2. 创建AscendJob

   ```go
   Create(ctx context.Context, job *v1.AscendJob, opts metav1.CreateOptions)(*v1.AscendJob, error)
   ```

   | Parameters | Input/Output | Parameter Type       | Description       |
   | ---------- | ------------ | -------------------- | ----------------- |
   | ctx        | Input        | context.Context      | 上下文，协程控制  |
   | job        | Input        | *v1.AscendJob        | AscendJob对象指针 |
   | opts       | Input        | metav1.CreateOptions | 创建选项          |
   | -          | Output       | *v1.AscendJob        | AscendJob对象指针 |
   | -          | Output       | error                | 错误信息          |

3. 获取AscendJob

   ```go
   Get(ctx context.Context, name string, opts metav1.GetOptions)(*v1.AscendJob, error)
   ```

   | Parameters | Input/Output | Parameter Type       | Description       |
   | ---------- | ------------ | -------------------- | ----------------- |
   | ctx        | Input        | context.Context      | 上下文，协程控制  |
   | name       | Input        | string               | AscendJob名称     |
   | opts       | Input        | metav1.GetOptions | 获取选项          |
   | -          | Output       | *v1.AscendJob        | AscendJob对象指针 |
   | -          | Output       | error                | 错误信息          |

4. 列举AscendJob

   ```go
   List(ctx context.Context, opts metav1.ListOptions)(*v1.AscendJobList, error)
   ```

   | Parameters | Input/Output | Parameter Type       | Description           |
   | ---------- | ------------ | -------------------- | --------------------- |
   | ctx        | Input        | context.Context      | 上下文，协程控制      |
   | opts       | Input        | metav1.ListOptions | 列举选项              |
   | -          | Output       | *v1.AscendJob        | AscendJobList对象指针 |
   | -          | Output       | error                | 错误信息              |

5. 观察AscendJob

   ```go
   Watch((ctx context.Context, opts metav1.ListOptions)(watch.Interface, error)
   ```

   | Parameters | Input/Output | Parameter Type     | Description      |
   | ---------- | ------------ | ------------------ | ---------------- |
   | ctx        | Input        | context.Context    | 上下文，协程控制 |
   | opts       | Input        | metav1.ListOptions | 列举选项         |
   | -          | Output       | watch.Interface    | watch类接口      |
   | -          | Output       | error              | 错误信息         |

6. 更新AscendJob

   ```go
   Update(ctx context.Context, job *v1.AscendJob, opts metav1.UpdateOptions)(*v1.AscendJob, error) 
   ```

   | Parameters | Input/Output | Parameter Type       | Description       |
   | ---------- | ------------ | -------------------- | ----------------- |
   | ctx        | Input        | context.Context      | 上下文，协程控制  |
   | job        | Input        | *v1.AscendJob        | AscendJob对象指针 |
   | opts       | Input        | metav1.UpdateOptions | 更新选项          |
   | -          | Output       | *v1.AscendJob        | AscendJob对象指针 |
   | -          | Output       | error                | 错误信息          |

7. 更新AscendJob状态

   ```go
   UpdateStatus(ctx context.Context, job *v1.AscendJob, opts metav1.UpdateOptions)(*v1.AscendJob, error)
   ```

   | Parameters | Input/Output | Parameter Type       | Description       |
   | ---------- | ------------ | -------------------- | ----------------- |
   | ctx        | Input        | context.Context      | 上下文，协程控制  |
   | job        | Input        | *v1.AscendJob        | AscendJob对象指针 |
   | opts       | Input        | metav1.UpdateOptions | 更新选项          |
   | -          | Output       | *v1.AscendJob        | AscendJob对象指针 |
   | -          | Output       | error                | 错误信息          |

8. 补丁AscendJob

   ```go
   Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*v1.AscendJob, error)
   ```

   | Parameters   | Input/Output | Parameter Type  | Description       |
   | ------------ | ------------ | --------------- | ----------------- |
   | ctx          | Input        | context.Context | 上下文，协程控制  |
   | name         | Input        | string          | AscendJob名称     |
   | pt           | Input        | types.PatchType | patch类型         |
   | data         | Input        | []byte          | patch信息         |
   | subresources | Input        | ...string       | 子信息            |
   | -            | Output       | *v1.AscendJob   | AscendJob对象指针 |
   | -            | Output       | error           | 错误信息          |

9. 删除AscendJob

   ```go
   Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
   ```

   | Parameters | Input/Output | Parameter Type       | Description      |
   | ---------- | ------------ | -------------------- | ---------------- |
   | ctx        | Input        | context.Context      | 上下文，协程控制 |
   | name       | Input        | string               | AscendJob名称    |
   | opts       | Input        | metav1.DeleteOptions | 删除选项         |
   | -          | Output       | error                | 错误信息         |

10. 批量删除AscendJob

    ```go
    DeleteCollection(ctx context.Context,opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
    ```

    | Parameters | Input/Output | Parameter Type       | Description      |
    | ---------- | ------------ | -------------------- | ---------------- |
    | ctx        | Input        | context.Context      | 上下文，协程控制 |
    | opts       | Input        | metav1.DeleteOptions | 删除选项         |
    | listOpts   | Input        | metav1.ListOptions   | 列举选项         |
    | -          | Output       | error                | 错误信息         |

11. 创建informerFactory

    ```go
    NewSharedInformerFactory(client versioned.Interface, defaultResync time.Duration) sharedInformerFactory
    ```

    | Parameters    | Input/Output | Parameter Type        | Description        |
    | ------------- | ------------ | --------------------- | ------------------ |
    | client        | Input        | versioned.Interface   | client类接口       |
    | defaultResync | Input        | time.Duration         | 默认的重新同步时间 |
    | -             | Output       | sharedInformerFactory | informer类接口     |

12. 创建informer

    ```go
    sharedInformerFactory.Batch().V1().Jobs().Informer()
    ```

    

