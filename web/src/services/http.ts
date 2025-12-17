import axios, { AxiosInstance, AxiosRequestConfig, AxiosResponse } from 'axios'
import { isKubeBindError, type KubeBindError, ErrorCodes } from '../types/binding'

// Custom error class for structured errors
export class StructuredError extends Error {
  public readonly kubeBindError: KubeBindError
  public readonly httpStatus: number

  constructor(kubeBindError: KubeBindError, httpStatus: number) {
    super(kubeBindError.message)
    this.kubeBindError = kubeBindError
    this.httpStatus = httpStatus
    this.name = 'StructuredError'
  }
}

class HttpClient {
  private client: AxiosInstance

  constructor() {
    this.client = axios.create({
      baseURL: '/api',
      timeout: 30000,
      withCredentials: true, // Enable cookies
      headers: {
        'Content-Type': 'application/json'
      }
    })

    this.setupInterceptors()
  }

  private setupInterceptors() {
    // Request interceptor - cookies are handled automatically with withCredentials: true
    // No need to manually add session parameters since HTTP-only cookies are sent automatically
    this.client.interceptors.request.use(
      (config) => {
        // Cookies are automatically included due to withCredentials: true setting
        return config
      },
      (error) => {
        return Promise.reject(error)
      }
    )

    // Response interceptor to handle structured errors
    this.client.interceptors.response.use(
      (response) => response,
      (error) => {
        const status = error.response?.status
        const responseData = error.response?.data

        // Check if we received a structured error from the backend
        if (responseData && isKubeBindError(responseData)) {
          // Handle specific error codes
          if (responseData.code === ErrorCodes.AUTHENTICATION_FAILED) {
            // Session expired or invalid - emit event for App.vue to handle
            console.warn('Authentication expired, need to re-authenticate')
            window.dispatchEvent(new CustomEvent('auth-expired'))
          }
          
          // Throw our custom structured error
          return Promise.reject(new StructuredError(responseData, status))
        }

        return Promise.reject(error)
      }
    )
  }

  // Generic HTTP methods
  async get<T = any>(url: string, config?: AxiosRequestConfig): Promise<AxiosResponse<T>> {
    return this.client.get<T>(url, config)
  }

  async post<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<AxiosResponse<T>> {
    return this.client.post<T>(url, data, config)
  }

  async put<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<AxiosResponse<T>> {
    return this.client.put<T>(url, data, config)
  }

  async delete<T = any>(url: string, config?: AxiosRequestConfig): Promise<AxiosResponse<T>> {
    return this.client.delete<T>(url, config)
  }

  // Method to make requests without auth headers (for public endpoints)
  async getPublic<T = any>(url: string, config?: AxiosRequestConfig): Promise<AxiosResponse<T>> {
    return axios.get<T>(`/api${url}`, config)
  }
}

export const httpClient = new HttpClient()