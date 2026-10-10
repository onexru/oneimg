package controllers

func bucketRemaining(capacity,usage uint64)uint64{
 if usage>=capacity {return 0}
 return capacity-usage
}
