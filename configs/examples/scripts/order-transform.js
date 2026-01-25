/**
 * Order Transformation Script
 *
 * This script calculates pricing with quantity-based discounts.
 * It demonstrates the transform function pattern for script filters.
 *
 * Usage in pipeline config:
 *   filters:
 *     - type: script
 *       scriptFile: ./configs/examples/scripts/order-transform.js
 */

function transform(record) {
    // Calculate discount based on quantity
    var discount = 0;
    if (record.quantity >= 10) {
        discount = 0.15; // 15% bulk discount
    } else if (record.quantity >= 5) {
        discount = 0.10; // 10% medium order discount
    } else {
        discount = 0.05; // 5% standard discount
    }

    // Calculate totals
    var subtotal = record.price * record.quantity;
    var discountAmount = subtotal * discount;
    var tax = (subtotal - discountAmount) * 0.08; // 8% tax

    // Return enriched record
    return {
        orderId: record.id,
        customerName: record.customer ? record.customer.name : 'Unknown',
        items: record.quantity,
        unitPrice: record.price,
        subtotal: subtotal,
        discountPercent: discount * 100,
        discountAmount: Math.round(discountAmount * 100) / 100,
        tax: Math.round(tax * 100) / 100,
        total: Math.round((subtotal - discountAmount + tax) * 100) / 100,
        processedAt: new Date().toISOString()
    };
}
