export interface OrderItem {
  name: string;
  priceCents: number;
  quantity: number;
}

export interface Order {
  merchant: string;
  orderNumber: string;
  orderDate: string;
  items: OrderItem[];
  totalCents: number;
  deliveryStatus: string;
}
