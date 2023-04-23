import sys
import os
import csv
import pandas as pd
import matplotlib.pyplot as plt
from prophet import Prophet
from sklearn.metrics import mean_absolute_error, mean_squared_error
import math

def load_csv_data(csv_file):
    timestamps = []
    values = []

    with open(csv_file, 'r', newline='') as csvfile:
        reader = csv.reader(csvfile)
        for row in reader:
            timestamp = pd.to_datetime(row[3], unit='s')
            timestamps.append(timestamp)
            values.append(float(row[4]))

    return pd.DataFrame({'ds': timestamps, 'y': values})

def process_workload(csv_file, workload_name):
    data = load_csv_data(csv_file)

    # Split the data into training (75%) and validation (25%) sets
    train_size = int(len(data) * 0.75)
    train_data = data.iloc[:train_size]
    validation_data = data.iloc[train_size:]

    # Train the Prophet model
    model = Prophet()
    model.fit(train_data)

    # Make predictions for the validation period
    future = model.make_future_dataframe(periods=len(validation_data), freq='30S')
    forecast = model.predict(future)

    # Plot the results
    fig, ax = plt.subplots()
    ax.plot(data['ds'], data['y'], label='Observed', linewidth=0.5)
    ax.plot(forecast['ds'], forecast['yhat'], label='Predicted', linewidth=0.5)
    ax.legend(loc='upper right')
    ax.set_title('Prophet Model Forecast vs Observed - ' + workload_name)
    ax.set_xlabel('Timestamp')
    ax.set_ylabel('CPU Usage')
    plt.xticks(rotation=45)
    plt.tight_layout()
    plt.savefig(f"plots/{workload_name}.png", dpi=1500)
    plt.close(fig)

    # Calculate accuracy metrics
    actual = validation_data['y'].values
    predicted = forecast.iloc[-len(validation_data):]['yhat'].values
    mae = mean_absolute_error(actual, predicted)
    mse = mean_squared_error(actual, predicted)
    rmse = math.sqrt(mse)

    return mae, mse, rmse

if len(sys.argv) != 2:
    print("Usage: python3 scriptname.py folder_path")
else:
    folder_path = sys.argv[1]
    output_file = "accuracy_metrics.csv"

    if not os.path.exists("plots"):
        os.mkdir("plots")

    with open(output_file, 'w', newline='') as csvfile:
        writer = csv.writer(csvfile)
        writer.writerow(['Workload', 'MAE', 'MSE', 'RMSE'])

        for file in os.listdir(folder_path):
            if file.endswith(".csv"):
                workload_name = file[:-4]
                csv_file = os.path.join(folder_path, file)
                mae, mse, rmse = process_workload(csv_file, workload_name)
                print(f"{workload_name}: MAE={mae:.2f}, MSE={mse:.2f}, RMSE={rmse:.2f}")
                writer.writerow([workload_name, mae, mse, rmse])

