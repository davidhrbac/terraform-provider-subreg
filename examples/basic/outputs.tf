output "zone_records" {
  description = "All records returned by subreg_dns_zone."
  value       = data.subreg_dns_zone.zone.records
}
