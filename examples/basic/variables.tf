variable "subreg_login" {
  type        = string
  description = "Subreg API login."
  sensitive   = true
}

variable "subreg_password" {
  type        = string
  description = "Subreg API password."
  sensitive   = true
}

variable "subreg_wsdl_url" {
  type        = string
  description = "Subreg WSDL URL."
  default     = "https://subreg.cz/wsdl"
}

variable "subreg_domain" {
  type        = string
  description = "Registered domain to manage."
}

variable "create_example_record" {
  type        = bool
  description = "Whether to create the example record."
  default     = false
}

variable "example_record_name" {
  type        = string
  description = "Record name without the domain."
  default     = "test"
}

variable "example_record_type" {
  type        = string
  description = "DNS record type."
  default     = "A"
}

variable "example_record_content" {
  type        = string
  description = "Record content value."
  default     = "203.0.113.10"
}

variable "example_record_prio" {
  type        = number
  description = "Priority for MX records."
  default     = null
}

variable "example_record_ttl" {
  type        = number
  description = "TTL in seconds."
  default     = null
}
