package models

func (bucket Buckets) CanStore(bytes uint64)bool{
 if bucket.Type=="default" || bucket.Type=="telegram" || bucket.Capacity==0{return true}
 return bucket.Usage<=bucket.Capacity && bytes<=bucket.Capacity-bucket.Usage
}
