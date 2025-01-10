```json
{
    "apiVersion": "kube-bind.io/v1alpha1",
    "kind": "ClusterBinding",
    "metadata": {
        "creationTimestamp": "2024-12-13T16:43:42Z",
        "generation": 1,
        "name": "cluster",
        "namespace": "kube-bind-9dstb",
        "resourceVersion": "3231",
        "uid": "4a652213-6264-4ade-b670-c4cc13de9f72"
    },
    "spec": {
        "kubeconfigSecretRef": {
            "key": "kubeconfig",
            "name": "kubeconfig"
        },
        "providerPrettyName": "BigCorp.com"
    },
    "status": {
        "conditions": [
            {
                "lastTransitionTime": "2024-12-13T16:43:54Z",
                "message": "Clocks of consumer cluster and service account cluster seem to be off by more than 10s",
                "reason": "HeartbeatTimeDrift",
                "severity": "Warning",
                "status": "False",
                "type": "Ready"
            },
            {
                "lastTransitionTime": "2024-12-13T16:43:54Z",
                "message": "Clocks of consumer cluster and service account cluster seem to be off by more than 10s",
                "reason": "HeartbeatTimeDrift",
                "severity": "Warning",
                "status": "False",
                "type": "Healthy"
            },
            {
                "lastTransitionTime": "2024-12-13T16:43:54Z",
                "status": "True",
                "type": "ValidVersion"
            }
        ],
        "heartbeatInterval": "5m0s",
        "konnectorVersion": "v0.3.0",
        "lastHeartbeatTime": "2024-12-13T16:56:24Z"
    }
}
```


```json
{
    "apiVersion": "kube-bind.io/v1alpha1",
    "kind": "APIServiceExport",
    "metadata": {
        "annotations": {
            "kube-bind.io/source-spec-hash": "nAE8b9oDQ7NjiP5gAtzmTpASVwEUuCLhAtupN"
        },
        "creationTimestamp": "2024-12-13T16:43:43Z",
        "generation": 1,
        "name": "mangodbs.mangodb.com",
        "namespace": "kube-bind-9dstb",
        "resourceVersion": "2048",
        "uid": "c2af9079-c886-443b-b7e7-8a4b17670daf"
    },
    "spec": {
        "group": "mangodb.com",
        "informerScope": "Namespaced",
        "names": {
            "kind": "MangoDB",
            "listKind": "MangoDBList",
            "plural": "mangodbs",
            "singular": "mangodb"
        },
        "scope": "Namespaced",
        "versions": [
            {
                "name": "v1alpha1",
                "schema": {
                    "openAPIV3Schema": {
                        "properties": {
                            "spec": {
                                "properties": {
                                    "backup": {
                                        "default": false,
                                        "type": "boolean"
                                    },
                                    "region": {
                                        "default": "us-east-1",
                                        "minLength": 1,
                                        "type": "string"
                                    },
                                    "tier": {
                                        "default": "Shared",
                                        "enum": [
                                            "Dedicated",
                                            "Shared"
                                        ],
                                        "type": "string"
                                    },
                                    "tokenSecret": {
                                        "minLength": 1,
                                        "type": "string"
                                    }
                                },
                                "required": [
                                    "tokenSecret"
                                ],
                                "type": "object"
                            },
                            "status": {
                                "properties": {
                                    "phase": {
                                        "enum": [
                                            "Pending",
                                            "Running",
                                            "Succeeded",
                                            "Failed",
                                            "Unknown"
                                        ],
                                        "type": "string"
                                    }
                                },
                                "type": "object"
                            }
                        },
                        "required": [
                            "spec"
                        ],
                        "type": "object"
                    }
                },
                "served": true,
                "storage": true,
                "subresources": {
                    "status": {}
                }
            }
        ]
    },
    "status": {
        "acceptedNames": {
            "kind": "MangoDB",
            "listKind": "MangoDBList",
            "plural": "mangodbs",
            "singular": "mangodb"
        },
        "conditions": [
            {
                "lastTransitionTime": "2024-12-13T16:43:54Z",
                "message": "APIServiceBinding mangodbs.mangodb.com in the consumer cluster does not have a SchemaInSync condition.",
                "reason": "Unknown",
                "severity": "Info",
                "status": "False",
                "type": "Ready"
            },
            {
                "lastTransitionTime": "2024-12-13T16:43:54Z",
                "status": "True",
                "type": "Connected"
            },
            {
                "lastTransitionTime": "2024-12-13T16:43:54Z",
                "message": "APIServiceBinding mangodbs.mangodb.com in the consumer cluster does not have a SchemaInSync condition.",
                "reason": "Unknown",
                "severity": "Info",
                "status": "False",
                "type": "ConsumerInSync"
            },
            {
                "lastTransitionTime": "2024-12-13T16:43:54Z",
                "message": "the initial names have been accepted",
                "reason": "InitialNamesAccepted",
                "status": "True",
                "type": "Established"
            },
            {
                "lastTransitionTime": "2024-12-13T16:43:54Z",
                "message": "no conflicts found",
                "reason": "NoConflicts",
                "status": "True",
                "type": "NamesAccepted"
            },
            {
                "lastTransitionTime": "2024-12-13T16:43:43Z",
                "status": "True",
                "type": "ProviderInSync"
            }
        ],
        "storedVersions": [
            "v1alpha1"
        ]
    }
}
```

```json
{
    "apiVersion": "kube-bind.io/v1alpha1",
    "kind": "APIServiceExportRequest",
    "metadata": {
        "creationTimestamp": "2024-12-13T16:43:42Z",
        "generation": 1,
        "name": "mangodbs.mangodb.com",
        "namespace": "kube-bind-9dstb",
        "resourceVersion": "2020",
        "uid": "90fe68a9-2431-4998-a197-c741b077799e"
    },
    "spec": {
        "resources": [
            {
                "group": "mangodb.com",
                "resource": "mangodbs"
            }
        ]
    },
    "status": {
        "conditions": [
            {
                "lastTransitionTime": "2024-12-13T16:43:43Z",
                "status": "True",
                "type": "Ready"
            },
            {
                "lastTransitionTime": "2024-12-13T16:43:43Z",
                "status": "True",
                "type": "ExportsReady"
            }
        ],
        "phase": "Succeeded"
    }
}
```

```json
{
    "apiVersion": "v1",
    "kind": "ServiceAccount",
    "metadata": {
        "creationTimestamp": "2024-12-13T16:43:42Z",
        "name": "kube-binder",
        "namespace": "kube-bind-9dstb",
        "resourceVersion": "2008",
        "uid": "4f3673af-0713-4719-a978-bf91b69c4081"
    }
}
{
    "apiVersion": "rbac.authorization.k8s.io/v1",
    "kind": "RoleBinding",
    "metadata": {
        "creationTimestamp": "2024-12-13T16:43:42Z",
        "name": "kube-binder",
        "namespace": "kube-bind-9dstb",
        "resourceVersion": "2007",
        "uid": "aca30246-a4d7-468a-9a20-ffdb72ebc0e5"
    },
    "roleRef": {
        "apiGroup": "rbac.authorization.k8s.io",
        "kind": "ClusterRole",
        "name": "kube-binder"
    },
    "subjects": [
        {
            "kind": "ServiceAccount",
            "name": "kube-binder",
            "namespace": "kube-bind-9dstb"
        }
    ]
}
{
    "apiVersion": "v1",
    "data": {
        "ca.crt": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURCVENDQWUyZ0F3SUJBZ0lJRDFya1FNVWFUK293RFFZSktvWklodmNOQVFFTEJRQXdGVEVUTUJFR0ExVUUKQXhNS2EzVmlaWEp1WlhSbGN6QWVGdzB5TkRFeU1UTXhOakkxTVRKYUZ3MHpOREV5TVRFeE5qTXdNVEphTUJVeApFekFSQmdOVkJBTVRDbXQxWW1WeWJtVjBaWE13Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLCkFvSUJBUUN3eFg4Tm5XQmkyVk1RVVQ5dWVkUFI3TjVvaUwrMkt3RXgxVmJrbDh2V3BmcFJvT0dsK1pXMmR0MGoKZlJaUFgrT2JLcjNqMW1IRE9jbnEyR3c3bTlENHBiSHBEeEFCM2g5ZDNpYVR6M1NubEttbW1jdzZRNzVGdnU0MApJa09Fd2ZDUS9oZUJWWE96ZWNRUmFGM29oMndjbXVGaVdValdSYnVIVmlnOG44bnRaem85S2lFeEFsV3k1TEpaCjhaTXliek12YXRrNjVJVGVQOWwxV0VMTmhKZkRIbjl3NUIxYkR1MlBYVTJreG9BSVhxamx0czJSOTAvOTE0d2gKS0FmQnMyZ2pGaVl1cUpzK1l3UVBzU1hHZFZCYnp6Vi9VWmNzWVRIVFM2V1E4Z3J2UDdadnFwUEZabWJBUDhFcQpmL0JZMDcrQ0tJb2VJZjdYcnJnT1NZQUs0OG1GQWdNQkFBR2pXVEJYTUE0R0ExVWREd0VCL3dRRUF3SUNwREFQCkJnTlZIUk1CQWY4RUJUQURBUUgvTUIwR0ExVWREZ1FXQkJUb2lmSnY3SU9oMjhZTGl3STJZdnREcGttU2xqQVYKQmdOVkhSRUVEakFNZ2dwcmRXSmxjbTVsZEdWek1BMEdDU3FHU0liM0RRRUJDd1VBQTRJQkFRQjdIeG9YUFFhdwpkb3cwdDUxMitDNjZISHorSUFrTVBRSXNwZ2IwSmdhWlNCRmM4NUpFVUpTRFgrVlp6SXpFVGxrUVdYSmZjd3oxCkRQLzhRcExXaUVwZm5nUDdidHR3cjRaZjYramVjTFFuODNTbTdBOTkrdTdLanFRRzEzTi9vanM2QURyUTFRbjAKcSttampqT1FIM2czdkRmTlEyZ1hOMHB6aTRjcG5kNDdSZmh4WFRTUE9rOFk3K25KOVV2aC9qSXNNQXU2UXVXTQpldDBrQ0xLTVRrSTBJZkI4ZXdOQUtYNTdMZmxuUWZZOWFmSVI3NXRJUmg4RmRmbHVicHUzeXBaNWJRaHBpcldJCk4yOHBhZ2ZWS1Z2M3Jvdzh5WCtLOTcvVDdaZVFEcTA3cGw1SURDMnNzK2R6d1orNCtIZ3RvUWFSN0p1S0tjajgKZWRqcU53Qnk2VmJPCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K",
        "namespace": "a3ViZS1iaW5kLTlkc3Ri",
        "token": "ZXlKaGJHY2lPaUpTVXpJMU5pSXNJbXRwWkNJNklrZDJaRU55VkU1R04yTjRSVkoxUjJwYVNsaEVlbWswWkVOSk5UTlNZVXR2VWxGdWVtOWFTRnAzUWxVaWZRLmV5SnBjM01pT2lKcmRXSmxjbTVsZEdWekwzTmxjblpwWTJWaFkyTnZkVzUwSWl3aWEzVmlaWEp1WlhSbGN5NXBieTl6WlhKMmFXTmxZV05qYjNWdWRDOXVZVzFsYzNCaFkyVWlPaUpyZFdKbExXSnBibVF0T1dSemRHSWlMQ0pyZFdKbGNtNWxkR1Z6TG1sdkwzTmxjblpwWTJWaFkyTnZkVzUwTDNObFkzSmxkQzV1WVcxbElqb2lhM1ZpWlMxaWFXNWtaWElpTENKcmRXSmxjbTVsZEdWekxtbHZMM05sY25acFkyVmhZMk52ZFc1MEwzTmxjblpwWTJVdFlXTmpiM1Z1ZEM1dVlXMWxJam9pYTNWaVpTMWlhVzVrWlhJaUxDSnJkV0psY201bGRHVnpMbWx2TDNObGNuWnBZMlZoWTJOdmRXNTBMM05sY25acFkyVXRZV05qYjNWdWRDNTFhV1FpT2lJMFpqTTJOek5oWmkwd056RXpMVFEzTVRrdFlUazNPQzFpWmpreFlqWTVZelF3T0RFaUxDSnpkV0lpT2lKemVYTjBaVzA2YzJWeWRtbGpaV0ZqWTI5MWJuUTZhM1ZpWlMxaWFXNWtMVGxrYzNSaU9tdDFZbVV0WW1sdVpHVnlJbjAuRm56TGhRRFF5Yzg3Y2E4aHJOZG9odVRTckxOMExTQ2lQWk04QnpxMWoyVHRmNjFQWE9kZXZFcTNna3gydUVYeWZ2UENNd09mYVVLakpHZjRheFlVa3FXLUxaZGNxb1ktRDBScjYxSjhlbWtRMzBJRmNrS1NPc2RrZUFuUE05UjNNdHpGYVJpbEJFb2lMbTM2RVBreXVVbVBRY1VfVVFScG9xTlh6bVEwM2VBZGtTN2VuanR2WHhNZTVLeUppWU5ZdmFqaVRTNUZCbHlQaFZRQUFGLWtzZWFXcElvZzY1cEJlbWtOMkk1cnREbUp2OEJ0a0w5VjVuSWYyRTZJRnBXaW80c2dteGg4V0NUbzRET1FNQ181aldxeHU2OVZYbjV6ckw0SXhEN3dkVnpRcUJXWlBxTV96blZPYVg5R3oyRTFtS1BaM3l6bHRVVWQtQkpJYnV4R2hB"
    },
    "kind": "Secret",
    "metadata": {
        "annotations": {
            "kubernetes.io/service-account.name": "kube-binder",
            "kubernetes.io/service-account.uid": "4f3673af-0713-4719-a978-bf91b69c4081"
        },
        "creationTimestamp": "2024-12-13T16:43:42Z",
        "labels": {
            "kubernetes.io/legacy-token-last-used": "2024-12-13"
        },
        "name": "kube-binder",
        "namespace": "kube-bind-9dstb",
        "resourceVersion": "2017",
        "uid": "6dd2e563-9a88-46b1-9814-87a6094884ae"
    },
    "type": "kubernetes.io/service-account-token"
}
{
    "apiVersion": "v1",
    "data": {
        "kubeconfig": "YXBpVmVyc2lvbjogdjEKY2x1c3RlcnM6Ci0gY2x1c3RlcjoKICAgIGNlcnRpZmljYXRlLWF1dGhvcml0eS1kYXRhOiBMUzB0TFMxQ1JVZEpUaUJEUlZKVVNVWkpRMEZVUlMwdExTMHRDazFKU1VSQ1ZFTkRRV1V5WjBGM1NVSkJaMGxKUkRGeWExRk5WV0ZVSzI5M1JGRlpTa3R2V2tsb2RtTk9RVkZGVEVKUlFYZEdWRVZVVFVKRlIwRXhWVVVLUVhoTlMyRXpWbWxhV0VwMVdsaFNiR042UVdWR2R6QjVUa1JGZVUxVVRYaE9ha2t4VFZSS1lVWjNNSHBPUkVWNVRWUkZlRTVxVFhkTlZFcGhUVUpWZUFwRmVrRlNRbWRPVmtKQlRWUkRiWFF4V1cxV2VXSnRWakJhV0UxM1oyZEZhVTFCTUVkRFUzRkhVMGxpTTBSUlJVSkJVVlZCUVRSSlFrUjNRWGRuWjBWTENrRnZTVUpCVVVOM2VGZzRUbTVYUW1reVZrMVJWVlE1ZFdWa1VGSTNUalZ2YVV3ck1rdDNSWGd4Vm1KcmJEaDJWM0JtY0ZKdlQwZHNLMXBYTW1SME1Hb0tabEphVUZnclQySkxjak5xTVcxSVJFOWpibkV5UjNjM2JUbEVOSEJpU0hCRWVFRkNNMmc1WkROcFlWUjZNMU51YkV0dGJXMWpkelpSTnpWR2RuVTBNQXBKYTA5RmQyWkRVUzlvWlVKV1dFOTZaV05SVW1GR00yOW9NbmRqYlhWR2FWZFZhbGRTWW5WSVZtbG5PRzQ0Ym5SYWVtODVTMmxGZUVGc1YzazFURXBhQ2poYVRYbGllazEyWVhSck5qVkpWR1ZRT1d3eFYwVk1UbWhLWmtSSWJqbDNOVUl4WWtSMU1sQllWVEpyZUc5QlNWaHhhbXgwY3pKU09UQXZPVEUwZDJnS1MwRm1Rbk15WjJwR2FWbDFjVXB6SzFsM1VWQnpVMWhIWkZaQ1lucDZWaTlWV21OeldWUklWRk0yVjFFNFozSjJVRGRhZG5Gd1VFWmFiV0pCVURoRmNRcG1MMEpaTURjclEwdEpiMlZKWmpkWWNuSm5UMU5aUVVzME9HMUdRV2ROUWtGQlIycFhWRUpZVFVFMFIwRXhWV1JFZDBWQ0wzZFJSVUYzU1VOd1JFRlFDa0puVGxaSVVrMUNRV1k0UlVKVVFVUkJVVWd2VFVJd1IwRXhWV1JFWjFGWFFrSlViMmxtU25ZM1NVOW9NamhaVEdsM1NUSlpkblJFY0d0dFUyeHFRVllLUW1kT1ZraFNSVVZFYWtGTloyZHdjbVJYU214amJUVnNaRWRXZWsxQk1FZERVM0ZIVTBsaU0wUlJSVUpEZDFWQlFUUkpRa0ZSUWpkSWVHOVlVRkZoZHdwa2IzY3dkRFV4TWl0RE5qWklTSG9yU1VGclRWQlJTWE53WjJJd1NtZGhXbE5DUm1NNE5VcEZWVXBUUkZnclZscDZTWHBGVkd4clVWZFlTbVpqZDNveENrUlFMemhSY0V4WGFVVndabTVuVURkaWRIUjNjalJhWmpZcmFtVmpURkZ1T0ROVGJUZEJPVGtyZFRkTGFuRlJSekV6VGk5dmFuTTJRVVJ5VVRGUmJqQUtjU3R0YW1wcVQxRklNMmN6ZGtSbVRsRXlaMWhPTUhCNmFUUmpjRzVrTkRkU1ptaDRXRlJUVUU5ck9GazNLMjVLT1ZWMmFDOXFTWE5OUVhVMlVYVlhUUXBsZERCclEweExUVlJyU1RCSlprSTRaWGRPUVV0WU5UZE1abXh1VVdaWk9XRm1TVkkzTlhSSlVtZzRSbVJtYkhWaWNIVXplWEJhTldKUmFIQnBjbGRKQ2s0eU9IQmhaMlpXUzFaMk0zSnZkemg1V0N0TE9UY3ZWRGRhWlZGRWNUQTNjR3cxU1VSRE1uTnpLMlI2ZDFvck5DdElaM1J2VVdGU04wcDFTMHRqYWpnS1pXUnFjVTUzUW5rMlZtSlBDaTB0TFMwdFJVNUVJRU5GVWxSSlJrbERRVlJGTFMwdExTMEsKICAgIHNlcnZlcjogaHR0cHM6Ly8xOTIuMTY4LjEyMi4xMDk6NDU1MjEKICBuYW1lOiBkZWZhdWx0CmNvbnRleHRzOgotIGNvbnRleHQ6CiAgICBjbHVzdGVyOiBkZWZhdWx0CiAgICBuYW1lc3BhY2U6IGt1YmUtYmluZC05ZHN0YgogICAgdXNlcjogZGVmYXVsdAogIG5hbWU6IGRlZmF1bHQKY3VycmVudC1jb250ZXh0OiBkZWZhdWx0CmtpbmQ6IENvbmZpZwpwcmVmZXJlbmNlczoge30KdXNlcnM6Ci0gbmFtZTogZGVmYXVsdAogIHVzZXI6CiAgICB0b2tlbjogZXlKaGJHY2lPaUpTVXpJMU5pSXNJbXRwWkNJNklrZDJaRU55VkU1R04yTjRSVkoxUjJwYVNsaEVlbWswWkVOSk5UTlNZVXR2VWxGdWVtOWFTRnAzUWxVaWZRLmV5SnBjM01pT2lKcmRXSmxjbTVsZEdWekwzTmxjblpwWTJWaFkyTnZkVzUwSWl3aWEzVmlaWEp1WlhSbGN5NXBieTl6WlhKMmFXTmxZV05qYjNWdWRDOXVZVzFsYzNCaFkyVWlPaUpyZFdKbExXSnBibVF0T1dSemRHSWlMQ0pyZFdKbGNtNWxkR1Z6TG1sdkwzTmxjblpwWTJWaFkyTnZkVzUwTDNObFkzSmxkQzV1WVcxbElqb2lhM1ZpWlMxaWFXNWtaWElpTENKcmRXSmxjbTVsZEdWekxtbHZMM05sY25acFkyVmhZMk52ZFc1MEwzTmxjblpwWTJVdFlXTmpiM1Z1ZEM1dVlXMWxJam9pYTNWaVpTMWlhVzVrWlhJaUxDSnJkV0psY201bGRHVnpMbWx2TDNObGNuWnBZMlZoWTJOdmRXNTBMM05sY25acFkyVXRZV05qYjNWdWRDNTFhV1FpT2lJMFpqTTJOek5oWmkwd056RXpMVFEzTVRrdFlUazNPQzFpWmpreFlqWTVZelF3T0RFaUxDSnpkV0lpT2lKemVYTjBaVzA2YzJWeWRtbGpaV0ZqWTI5MWJuUTZhM1ZpWlMxaWFXNWtMVGxrYzNSaU9tdDFZbVV0WW1sdVpHVnlJbjAuRm56TGhRRFF5Yzg3Y2E4aHJOZG9odVRTckxOMExTQ2lQWk04QnpxMWoyVHRmNjFQWE9kZXZFcTNna3gydUVYeWZ2UENNd09mYVVLakpHZjRheFlVa3FXLUxaZGNxb1ktRDBScjYxSjhlbWtRMzBJRmNrS1NPc2RrZUFuUE05UjNNdHpGYVJpbEJFb2lMbTM2RVBreXVVbVBRY1VfVVFScG9xTlh6bVEwM2VBZGtTN2VuanR2WHhNZTVLeUppWU5ZdmFqaVRTNUZCbHlQaFZRQUFGLWtzZWFXcElvZzY1cEJlbWtOMkk1cnREbUp2OEJ0a0w5VjVuSWYyRTZJRnBXaW80c2dteGg4V0NUbzRET1FNQ181aldxeHU2OVZYbjV6ckw0SXhEN3dkVnpRcUJXWlBxTV96blZPYVg5R3oyRTFtS1BaM3l6bHRVVWQtQkpJYnV4R2hBCg=="
    },
    "kind": "Secret",
    "metadata": {
        "creationTimestamp": "2024-12-13T16:43:42Z",
        "name": "kubeconfig",
        "namespace": "kube-bind-9dstb",
        "resourceVersion": "2016",
        "uid": "43a0f6d7-ebb5-4562-8ae2-536b87c3c3c0"
    },
    "type": "Opaque"
}
```

