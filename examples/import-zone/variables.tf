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
  description = "Registered domain to import."
}
